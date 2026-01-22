#!/usr/bin/env python3
# pylint: disable=missing-function-docstring,missing-module-docstring
# flake8: noqa: E501

import subprocess
import argparse
import os
import sys

default_namespace: str = "integration-tests"
gateway_api: str = (
    "https://github.com/kubernetes-sigs/gateway-api/"
    "releases/download/v1.4.1/standard-install.yaml"
)
sail_repo: str = "https://istio-ecosystem.github.io/sail-operator"


def get_istio_version() -> str:
    """Get ISTIO_VERSION from environment, required for cluster setup operations."""
    istio_version = os.environ.get("ISTIO_VERSION")
    if not istio_version:
        print("ERROR: ISTIO_VERSION environment variable is required", file=sys.stderr)
        print("Please set ISTIO_VERSION to the desired Istio version (e.g., 1.28.2)", file=sys.stderr)
        print("You can set a default in the Makefile or export it in your shell", file=sys.stderr)
        sys.exit(1)
    return istio_version


def run(cmd: str, check: bool = True) -> subprocess.CompletedProcess[bytes]:
    return subprocess.run(cmd, shell=True, check=check)


def get_kind_context(name: str) -> str:
    return f"kind-{name}"


def build_images() -> None:
    print("Building container images")
    run("make build.image")


def create_cluster(name: str) -> None:
    print(f"Creating kind cluster: {name}")
    result = run(f"kind get clusters | grep -q '^{name}$'", check=False)
    if result.returncode == 0:
        print(f"Cluster {name} already exists, skipping creation")
    else:
        run(f"kind create cluster --name {name}")


def load_images(name: str) -> None:
    print(f"Loading images into kind cluster: {name}")
    run("make cluster.load-images")


def deploy_gateway_api_crds(context: str) -> None:
    print("Deploying Gateway API CRDs")
    run(f"kubectl --context {context} apply -f {gateway_api}")


def deploy_istio_sail(context: str) -> None:
    istio_version = get_istio_version()

    print("Deploying Istio Sail Operator")
    run(f"helm repo add sail-operator {sail_repo}")
    run("helm repo update")
    run(f"kubectl --context {context} create namespace sail-operator", check=False)

    result = run(
        f"helm list --namespace sail-operator --kube-context {context} "
        f"-o json | grep -q sail-operator",
        check=False
    )

    if result.returncode != 0:
        run(
            f"helm install sail-operator sail-operator/sail-operator "
            f"--version {istio_version} "
            f"--namespace sail-operator --kube-context {context}"
        )
    else:
        print("Sail operator already installed, skipping")

    run(
        f"kubectl --context {context} wait --for=condition=Available "
        f"deployment/sail-operator -n sail-operator --timeout=300s"
    )


def create_gateway_class(context: str) -> None:
    print("Creating GatewayClass for Istio")
    gateway_class = """
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: istio
spec:
  controllerName: istio.io/gateway-controller
"""
    run(
        f"echo '{gateway_class}' | kubectl "
        f"--context {context} apply -f -"
    )


def create_gateway(context: str) -> None:
    run(
        f"kubectl --context {context} "
        f" create namespace {default_namespace}", check=False
    )

    print("Creating Gateway for Istio")
    run(f"kubectl --context {context} -n {default_namespace} apply -f config/samples/gateway.yaml")

    run(
        f"kubectl --context {context} -n {default_namespace} wait "
        "--for=condition=Programmed gateway/coraza-gateway --timeout=300s"
    )


def create_istio_control_plane(context: str) -> None:
    istio_version = get_istio_version()

    run(
        f"kubectl --context {context} "
        " create namespace coraza-system", check=False
    )

    print("Creating Istio control-plane")
    istio = f"""
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  namespace: coraza-system
  name: coraza
spec:
  namespace: coraza-system
  version: v{istio_version}
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
    run(
        f"echo '{istio}' | kubectl "
        f"--context {context} apply -f -"
    )
    run(
        f"kubectl --context {context} --namespace coraza-system wait "
        "--for=condition=Ready istio/coraza --timeout=300s"
    )


def deploy_coraza_operator(context: str) -> None:
    print("Deploying Coraza Operator")

    result = run(
        f"kubectl --context {context} --namespace coraza-system "
        f"get deployment coraza-controller-manager 2>/dev/null",
        check=False
    )
    deployment_exists = result.returncode == 0

    run(f"kubectl --context {context} apply -k config/default")

    if deployment_exists:
        print("Restarting existing controller-manager deployment to pick up any image updates")
        run(
            f"kubectl --context {context} --namespace coraza-system "
            f"rollout restart deployment/coraza-controller-manager"
        )

    run(
        f"kubectl --context {context} --namespace coraza-system wait "
        "--for=condition=Available "
        "deployment/coraza-controller-manager --timeout=300s"
    )


def delete_cluster(name: str) -> None:
    print(f"Deleting kind cluster: {name}")
    run(f"kind delete cluster --name {name}", check=False)


def setup_cluster(name: str) -> None:
    build_images()
    create_cluster(name)
    load_images(name)

    context = get_kind_context(name)

    deploy_gateway_api_crds(context)
    deploy_istio_sail(context)
    create_istio_control_plane(context)
    create_gateway_class(context)
    create_gateway(context)
    deploy_coraza_operator(context)

    print("Cluster setup complete")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["create", "delete", "setup"])
    parser.add_argument("--name", default="coraza-kubernetes-operator-integration")
    args = parser.parse_args()

    if args.action == "create":
        create_cluster(args.name)
    elif args.action == "setup":
        setup_cluster(args.name)
    elif args.action == "delete":
        delete_cluster(args.name)


if __name__ == "__main__":
    main()
