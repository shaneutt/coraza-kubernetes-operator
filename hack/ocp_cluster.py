#!/usr/bin/env python3
import os
import subprocess
import sys
import time
import argparse
import shutil
from pathlib import Path

def run(cmd, input_str=None, capture_output=True, check=True):
    """Unified execution helper with logging"""
    print(f"+ {cmd}")
    try:
        result = subprocess.run(
            cmd, shell=True, check=check, text=True,
            input=input_str, capture_output=capture_output
        )
        if result.stdout and not capture_output:
            print(result.stdout.strip())
        return result
    except subprocess.CalledProcessError as e:
        print(f"ERROR: {e.stderr if e.stderr else e}")
        if check: sys.exit(e.returncode)
        return e

def get_istio_version(args) -> str:
    version = args.istio_version or os.environ.get("ISTIO_VERSION") or "v1.29.0"
    if not version.startswith('v'):
        version = f"v{version}"
    return version

def setup_internal_registry(args):
    print(f"--- Setting up OCP Internal Registry in {args.coraza_ns} ---")
    run("oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{\"spec\":{\"defaultRoute\":true}}' --type=merge")
    url = ""
    start = time.time()
    while time.time() - start < args.timeout:
        res = run("oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}'", check=False)
        if res.returncode == 0:
            url = res.stdout.strip()
            break
        time.sleep(5)

    run(f"oc create namespace {args.coraza_ns} --dry-run=client -o yaml | oc apply -f -")

    rb_yaml = f"""
kind: List
apiVersion: v1
items:
- apiVersion: rbac.authorization.k8s.io/v1
  kind: RoleBinding
  metadata: {{name: image-puller, namespace: {args.coraza_ns}}}
  roleRef: {{apiGroup: rbac.authorization.k8s.io, kind: ClusterRole, name: system:image-puller}}
  subjects:
  - {{kind: Group, apiGroup: rbac.authorization.k8s.io, name: 'system:unauthenticated'}}
  - {{kind: Group, apiGroup: rbac.authorization.k8s.io, name: 'system:serviceaccounts'}}
- apiVersion: rbac.authorization.k8s.io/v1
  kind: RoleBinding
  metadata: {{name: image-pusher, namespace: {args.coraza_ns}}}
  roleRef: {{apiGroup: rbac.authorization.k8s.io, kind: ClusterRole, name: system:image-builder}}
  subjects: [{{kind: Group, apiGroup: rbac.authorization.k8s.io, name: 'system:unauthenticated'}}]
"""
    run("oc apply -f -", input_str=rb_yaml)

def deploy_ocp_metallb(args):
    print("--- Deploying MetalLB ---")
    run(f"oc create namespace metallb-system --dry-run=client -o yaml | oc apply -f -")
    run("oc delete operatorgroup -n metallb-system --all", check=False)

    run("oc apply -f -", input_str="apiVersion: operators.coreos.com/v1\nkind: OperatorGroup\nmetadata: {name: metallb-operatorgroup, namespace: metallb-system}\nspec: {}")
    run("oc apply -f -", input_str="apiVersion: operators.coreos.com/v1alpha1\nkind: Subscription\nmetadata: {name: metallb-operator-sub, namespace: metallb-system}\nspec: {channel: stable, name: metallb-operator, source: redhat-operators, sourceNamespace: openshift-marketplace}")

    print("Waiting for MetalLB CSV to succeed...")
    run(f"timeout {args.timeout}s bash -c 'until oc get csv -n metallb-system 2>/dev/null | grep -q Succeeded; do sleep 5; done'")
    run("oc apply -f -", input_str="apiVersion: metallb.io/v1beta1\nkind: MetalLB\nmetadata: {name: metallb, namespace: metallb-system}")

    ips_res = run("oc get nodes -l node-role.kubernetes.io/worker -o jsonpath='{.items[*].status.addresses[?(@.type==\"InternalIP\")].address}'")
    ips = ips_res.stdout.strip().split()
    metallb_configs = f"""
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata: {{name: default, namespace: metallb-system}}
spec: {{addresses: {[f"{ip}-{ip}" for ip in ips]}}}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata: {{name: default, namespace: metallb-system}}
spec: {{ipAddressPools: ["default"]}}
"""
    run("oc apply -f -", input_str=metallb_configs)
    return True

def deploy_sail_operator(args):
    print("--- Deploying Sail Operator ---")
    if Path("sail-operator").exists(): run("rm -rf sail-operator")
    run(f"git clone --depth 1 --branch main {args.sail_repo_url}")

    os.chdir("sail-operator")
    run("make deploy")
    run(f"oc -n sail-operator wait --for=condition=Available deployment/sail-operator --timeout={args.timeout}s")
    os.chdir(args.working_dir)

    print("--- Cleaning up sail-operator folder ---")
    run("rm -rf sail-operator")

def deploy_gateway_class(args):
    print("--- Creating GatewayClass ---")
    run("oc apply -f -", input_str="apiVersion: gateway.networking.k8s.io/v1\nkind: GatewayClass\nmetadata: {name: istio}\nspec: {controllerName: istio.io/gateway-controller}")

def create_istio_resources(args, version):
    print(f"--- Creating Istio Control Plane ({version}) ---")
    run(f"oc create namespace {args.coraza_ns} --dry-run=client -o yaml | oc apply -f -")
    istio_cr = f"""
apiVersion: sailoperator.io/v1
kind: Istio
metadata: {{namespace: {args.coraza_ns}, name: coraza}}
spec:
  namespace: {args.coraza_ns}
  version: {version}
  values:
    pilot:
      env:
        PILOT_GATEWAY_API_CONTROLLER_NAME: "istio.io/gateway-controller"
        PILOT_ENABLE_GATEWAY_API: "true"
        PILOT_ENABLE_GATEWAY_API_STATUS: "true"
        PILOT_ENABLE_ALPHA_GATEWAY_API: "false"
        PILOT_ENABLE_GATEWAY_API_DEPLOYMENT_CONTROLLER: "true"
        PILOT_ENABLE_GATEWAY_API_GATEWAYCLASS_CONTROLLER: "false"
        PILOT_GATEWAY_API_DEFAULT_GATEWAYCLASS_NAME: "istio"
        PILOT_MULTI_NETWORK_DISCOVER_GATEWAY_API: "false"
        ENABLE_GATEWAY_API_MANUAL_DEPLOYMENT: "false"
        PILOT_ENABLE_GATEWAY_API_CA_CERT_ONLY: "true"
        PILOT_ENABLE_GATEWAY_API_COPY_LABELS_ANNOTATIONS: "false"
"""
    run("oc apply -f -", input_str=istio_cr)
    run(f"oc wait --for=condition=Ready istio/coraza -n {args.coraza_ns} --timeout={args.timeout}s")

def deploy_coraza_operator(args):
    print(f"--- Deploying Coraza Operator ---")
    
    project_root = Path(__file__).parent.parent.absolute()
    os.chdir(project_root)
    run("make build.image")

    # --- REGISTRY ---
    res = run("oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}'")
    registry_host_external = res.stdout.strip()
    push_image = f"{registry_host_external}/{args.coraza_ns}/coraza-operator:dev"
    
    pull_image = f"image-registry.openshift-image-registry.svc:5000/{args.coraza_ns}/coraza-operator:dev"

    print(f"Logging in to OpenShift registry at {registry_host_external}...")
    run(f"docker login -u kubeadmin -p $(oc whoami -t) {registry_host_external}")
    
    print(f"Tagging and Pushing image to external route: {push_image}...")
    run(f"docker tag ghcr.io/networking-incubator/coraza-kubernetes-operator:dev {push_image}")
    run(f"docker push {push_image}")

    print(f"Updating manifests to pull from internal registry: {pull_image}...")
    os.chdir(project_root / "config" / "default")
    run(f"kustomize edit set image ghcr.io/networking-incubator/coraza-kubernetes-operator:dev={pull_image}")
    
    os.chdir(project_root)
    run("oc apply -k config/default")

    # --- THE SCC PATCH ---
    print("Patching deployment to remove incompatible hardcoded SCC values...")
    patch = "'[{\"op\": \"remove\", \"path\": \"/spec/template/spec/securityContext/runAsUser\"}, {\"op\": \"remove\", \"path\": \"/spec/template/spec/securityContext/fsGroup\"}, {\"op\": \"remove\", \"path\": \"/spec/template/spec/securityContext/seccompProfile\"}]'"
    
    run(f"oc patch deployment coraza-controller-manager -n {args.coraza_ns} --type=json -p={patch}", check=False)

    print(f"Waiting for Coraza Operator to become Available (Timeout: {args.timeout}s)...")
    run(f"oc wait --for=condition=Available deployment/coraza-controller-manager -n {args.coraza_ns} --timeout={args.timeout}s")

def create_gateway(args, use_lb):
    print(f"--- Creating Gateway in {args.test_ns} ---")
    run(f"oc create namespace {args.test_ns} --dry-run=client -o yaml | oc apply -f -")
    project_root = Path(__file__).parent.parent.absolute()
    gw_path = project_root / "config" / "samples" / "gateway.yaml"
    
    if use_lb:
        run(f"oc apply -f {gw_path} -n {args.test_ns}")
    else:
        run(f"oc annotate -f {gw_path} networking.istio.io/service-type=ClusterIP --local -o yaml | oc apply -f - -n {args.test_ns}")
    
    run(f"oc wait --for=condition=Programmed gateway/coraza-gateway -n {args.test_ns} --timeout={args.timeout}s")

def run_integration_tests():
    print("\n--- Running Coraza Integration Tests ---")
    project_root = Path(__file__).parent.parent.absolute()
    os.chdir(project_root)
    # We set capture_output=False so the user sees the 'go test' progress live
    run("go test -v -tags=integration ./test/integration/...", capture_output=False)

def main():
    parser = argparse.ArgumentParser(
        description="Coraza OCP Integration Setup: Automates deployment of MetalLB, Sail Operator, and Istio on OpenShift.",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
        epilog="Priority Logic: CLI arguments override Environment Variables, which override hardcoded defaults."
    )
    parser.add_argument("action", choices=["setup", "cleanup", "test", "setup-test"], 
                        help="Action to perform: 'setup' for deploy only, 'test' for tests only, 'setup-test' for both.")
    
    parser.add_argument("--coraza-ns", 
                        default=os.getenv("CORAZA_NS", "coraza-system"),
                        help="Primary namespace for Coraza Operator and Istio Control Plane. (Env: CORAZA_NS)")
    
    parser.add_argument("--test-ns", 
                        default=os.getenv("TEST_NS", "integration-tests"),
                        help="Namespace where the test gateway and sample apps are deployed. (Env: TEST_NS)")
    
    parser.add_argument("--istio-version", 
                        default=os.getenv("ISTIO_VERSION", "v1.29.0"),
                        help="Istio version string. Must be supported by the Sail Operator catalog. (Env: ISTIO_VERSION)")
    
    parser.add_argument("--timeout", 
                        type=int, 
                        default=int(os.getenv("TIMEOUT", 300)),
                        help="Seconds to wait for deployments and CSVs to become ready. (Env: TIMEOUT)")
    
    parser.add_argument("--sail-repo-url", 
                        default=os.getenv("SAIL_REPO_URL", "https://github.com/istio-ecosystem/sail-operator.git"),
                        help="Git URL for the Sail Operator repository. (Env: SAIL_REPO_URL)")

    parser.add_argument("--deploy-metallb", action="store_true", 
                        default=False,
                        help="Whether to deploy and configure MetalLB for LoadBalancer support.")
    
    parser.add_argument("--working-dir", 
                        default=os.getenv("WORKING_DIR", Path.cwd()),
                        help="Base directory for temporary clones and file path resolution. (Env: WORKING_DIR)")

    args = parser.parse_args()
    args.working_dir = Path(args.working_dir)

    if args.action in ["setup", "setup-test"]:
        ver = get_istio_version(args)
        setup_internal_registry(args)
        if args.deploy_metallb: deploy_ocp_metallb(args)      
        deploy_sail_operator(args)
        deploy_gateway_class(args)
        create_istio_resources(args, ver)
        deploy_coraza_operator(args)
        create_gateway(args, use_lb=args.deploy_metallb)
        print("\n=======================================================")
        print("✅ SUCCESS! Coraza Operator and Istio are ready on OCP!")
        print("=======================================================")

    if args.action in ["test", "setup-test"]:
        run_integration_tests()
        print("\n=======================================================")
        print("Integration Test Execution Completed")
        print("=======================================================")
        
    elif args.action == "cleanup":
        print("\n=======================================================")
        print("--- Initiating Cleanup ---")
        print("=======================================================")
        
        project_root = Path(__file__).parent.parent.absolute()

        print("Cleaning up Coraza WAF instances (clearing finalizers)...")
        run("oc delete engines.waf.k8s.coraza.io --all -A", check=False)
        run("oc delete rulesets.waf.k8s.coraza.io --all -A", check=False)

        print("Cleaning up Istio control planes...")
        run(f"oc delete istio --all -n {args.coraza_ns}", check=False)

        print("Removing Coraza Operator and cluster-scoped RBAC/CRDs...")
        os.chdir(project_root)
        run("oc delete -k config/default", check=False)

        print("Removing GatewayClasses...")
        run("oc delete gatewayclass istio", check=False)

        namespaces = f"{args.coraza_ns} {args.test_ns} sail-operator"
        if args.deploy_metallb:
            namespaces += " metallb-system"

        run(f"oc delete ns {namespaces}", check=False)
        
        print("\n✅ Cleanup completed!")

if __name__ == "__main__":
    main()