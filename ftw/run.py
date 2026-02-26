#!/usr/bin/env python3
import argparse
import subprocess
import json
import sys
import os
import threading
import time
import socket
import urllib.request
import urllib.error
import tempfile
import yaml
from contextlib import closing


class KubeHelper:
    """Helper class to manage kubectl operations with consistent namespace and kubeconfig."""

    def __init__(self, namespace, kubeconfig):
        self.namespace = namespace
        self.kubeconfig = kubeconfig

    def kubectl(self, *args, capture_output=True, text=True, check=True):
        """
        Execute kubectl command with the configured namespace and kubeconfig.

            *args: kubectl command arguments.
            capture_output (bool): Whether to capture stdout/stderr (passed to subprocess.run).
            text (bool): Whether to decode output as text (passed to subprocess.run).
            check (bool): Whether to raise CalledProcessError on non-zero exit (passed to subprocess.run).
        Returns:
            subprocess.CompletedProcess: Result of the kubectl command.

        """
        cmd = [
            "kubectl",
            "--kubeconfig", self.kubeconfig,
            "-n", self.namespace
        ] + list(args)

        return subprocess.run(cmd, capture_output=capture_output, text=text, check=check)

    def get_gateway_service_info(self, gateway_name):
        """
        Get the service associated with a gateway and extract its IP/type, port, and name.

        Returns:
            tuple: (ip_or_type, port, service_name) where:
                   - ip_or_type is either the LoadBalancer IP or "ClusterIP"
                   - port is the HTTP port number
                   - service_name is the name of the service
        """
        try:
            result = self.kubectl(
                "get", "services",
                "-l", f"gateway.networking.k8s.io/gateway-name={gateway_name}",
                "-o", "json"
            )
            data = json.loads(result.stdout)
        except subprocess.CalledProcessError as e:
            print(f"Error executing kubectl: {e.stderr}", file=sys.stderr)
            sys.exit(1)
        except json.JSONDecodeError as e:
            print(f"Error parsing kubectl output: {e}", file=sys.stderr)
            sys.exit(1)

        # Check that we have exactly one service
        items = data.get("items", [])
        if len(items) == 0:
            print(f"Error: No service found with label gateway.networking.k8s.io/gateway-name={gateway_name}", file=sys.stderr)
            sys.exit(1)
        elif len(items) > 1:
            print(f"Error: Multiple services found with label gateway.networking.k8s.io/gateway-name={gateway_name}, expected only one", file=sys.stderr)
            sys.exit(1)

        service = items[0]
        service_name = service.get("metadata", {}).get("name", "")

        # Determine IP or type
        service_type = service.get("spec", {}).get("type", "")
        ip_or_type = "ClusterIP"

        if service_type == "LoadBalancer":
            ingress_list = service.get("status", {}).get("loadBalancer", {}).get("ingress", [])
            if ingress_list and len(ingress_list) > 0:
                # Get IP from first ingress entry
                ip_or_type = ingress_list[0].get("ip", "ClusterIP")

        # Determine port
        ports = service.get("spec", {}).get("ports", [])
        port = 80  # Default

        for port_entry in ports:
            if port_entry.get("name") == "http":
                port = port_entry.get("port", 80)
                break

        return ip_or_type, port, service_name

    def port_forward(self, service_name, local_port, service_port):
        """
        Create a port-forward to a service (blocking call, should be run in a thread).

        Args:
            service_name: Name of the service
            local_port: Local port to forward to
            service_port: Service port to forward from
        """
        self.kubectl(
            "port-forward",
            f"service/{service_name}",
            f"{local_port}:{service_port}",
            capture_output=False,
            check=False
        )

    def stream_pod_logs(self, label_selector, output_file):
        """
        Stream logs from pods matching a label selector to a file (blocking call, should be run in a thread).

        Args:
            label_selector: Label selector to find the pods (e.g., "gateway.networking.k8s.io/gateway-name=mygateway")
            output_file: File path to write logs to
        """
        with open(output_file, 'w', buffering=1) as f:
            process = subprocess.Popen(
                [
                    "kubectl",
                    "--kubeconfig", self.kubeconfig,
                    "-n", self.namespace,
                    "logs",
                    "-l", label_selector,
                    "-f",
                    "--all-containers=true"
                ],
                stdout=f,
                stderr=subprocess.STDOUT,
                text=True
            )
            process.wait()


def find_free_port():
    """Find a free port on localhost."""
    with closing(socket.socket(socket.AF_INET, socket.SOCK_STREAM)) as s:
        s.bind(('127.0.0.1', 0))
        s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        return s.getsockname()[1]


def test_connectivity(host, port, max_retries=30, retry_delay=1):
    """
    Test connectivity to a host:port combination.

    Args:
        host: Host to connect to
        port: Port to connect to
        max_retries: Maximum number of connection attempts
        retry_delay: Delay between retries in seconds

    Returns:
        bool: True if connection successful, False otherwise
    """
    url = f"http://{host}:{port}"

    for attempt in range(max_retries):
        try:
            # Try to make a simple HTTP request
            with urllib.request.urlopen(url, timeout=2) as response:
                status_code = response.getcode()
                print(f"Connectivity test successful: {url} (status: {status_code})")
                return True
        except (urllib.error.URLError, OSError) as e:
            if attempt < max_retries - 1:
                print(f"Connection attempt {attempt + 1}/{max_retries} failed, retrying in {retry_delay}s...")
                time.sleep(retry_delay)
            else:
                print(f"Connection failed after {max_retries} attempts")
                return False
        except Exception as e:
            print(f"Unexpected error during connectivity test: {e}")
            if attempt < max_retries - 1:
                time.sleep(retry_delay)
            else:
                return False

    return False


def main():
    parser = argparse.ArgumentParser(description="FTW test runner for Kubernetes Gateway")
    parser.add_argument("--namespace", required=True, help="Kubernetes namespace")
    parser.add_argument("--gateway", required=True, help="Gateway name")
    parser.add_argument("--config-file", required=True, help="FTW configuration file")
    parser.add_argument("--rules-directory", required=True, help="Rules directory")
    parser.add_argument("--kubeconfig", required=True, help="Kubeconfig file location")
    parser.add_argument("--output-log", required=False, help="Output for execution log. If empty will output to stdout")
    parser.add_argument("--output-format", required=False, help="Output format for execution log. If empty will use the default")

    args = parser.parse_args()

    # Initialize Kubernetes helper
    kube = KubeHelper(args.namespace, args.kubeconfig)

    # Get service information
    ip_or_type, port, service_name = kube.get_gateway_service_info(args.gateway)

    print(f"Service Name: {service_name}")
    print(f"Service IP/Type: {ip_or_type}")
    print(f"Service Port: {port}")

    # Determine target host and port for testing
    port_forward_process = None
    target_host = ip_or_type
    target_port = port

    try:
        # Set up port-forward if ClusterIP
        if ip_or_type == "ClusterIP":
            print("\nService is ClusterIP, setting up port-forward...")
            local_port = find_free_port()
            print(f"Using local port {local_port} for port-forward")

            # Start port-forward in a subprocess (not thread, so we can terminate it)
            port_forward_process = subprocess.Popen(
                [
                    "kubectl",
                    "--kubeconfig", args.kubeconfig,
                    "-n", args.namespace,
                    "port-forward",
                    f"service/{service_name}",
                    f"{local_port}:{port}"
                ],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE
            )

            # Update target for connectivity test
            target_host = "127.0.0.1"
            target_port = local_port

            # Give port-forward a moment to start
            time.sleep(2)

        # Test connectivity
        print(f"\nTesting connectivity to {target_host}:{target_port}...")
        if not test_connectivity(target_host, target_port):
            print("ERROR: Could not establish connectivity to the gateway", file=sys.stderr)
            sys.exit(1)

        print("\n" + "="*60)
        print("Gateway is ready for testing")
        print(f"Target: {target_host}:{target_port}")
        print("="*60 + "\n")

        # Set up log streaming
        log_file = tempfile.NamedTemporaryFile(mode='w', prefix='ftw_logs_', suffix='.log', delete=False)
        log_filename = log_file.name
        log_file.close()  # Close it so kubectl can write to it

        print(f"Streaming pod logs to: {log_filename}")

        log_thread = threading.Thread(
            target=kube.stream_pod_logs,
            args=(f"gateway.networking.k8s.io/gateway-name={args.gateway}", log_filename),
            daemon=True
        )
        log_thread.start()

        # Wait for log streaming to start with a readiness check and configurable timeout
        log_start_timeout = float(os.getenv("FTW_LOG_START_TIMEOUT_SECONDS", "5"))
        log_start_deadline = time.time() + log_start_timeout
        while time.time() < log_start_deadline:
            try:
                if os.path.exists(log_filename) and os.path.getsize(log_filename) > 0:
                    break
            except OSError:
                # File may not be ready yet; ignore and retry
                pass
            time.sleep(0.1)
        if not os.path.exists(log_filename) or os.path.getsize(log_filename) == 0:
            print(
                f"Warning: log file {log_filename} not initialized after {log_start_timeout} seconds; "
                "continuing without confirmed log streaming.",
                file=sys.stderr,
            )

        # Load and modify the config file to add runtime overrides
        try:
            with open(args.config_file, 'r') as f:
                config = yaml.safe_load(f) or {}
        except FileNotFoundError:
            print(f"ERROR: Config file not found: {args.config_file}", file=sys.stderr)
            sys.exit(1)
        except PermissionError:
            print(f"ERROR: Permission denied reading config file: {args.config_file}", file=sys.stderr)
            sys.exit(1)
        except yaml.YAMLError as e:
            print(f"ERROR: Invalid YAML in config file {args.config_file}: {e}", file=sys.stderr)
            sys.exit(1)
        except Exception as e:
            print(f"ERROR: Failed to load config file {args.config_file}: {e}", file=sys.stderr)
            sys.exit(1)

        # Add input settings under testoverride
        if 'testoverride' not in config:
            config['testoverride'] = {}
        if 'input' not in config['testoverride']:
            config['testoverride']['input'] = {}

        config['testoverride']['input']['dest_addr'] = target_host
        config['testoverride']['input']['port'] = target_port

        # Write modified config to a temporary file
        try:
            modified_config_file = tempfile.NamedTemporaryFile(mode='w', prefix='ftw_config_', suffix='.yaml', delete=False)
            yaml.dump(config, modified_config_file)
            modified_config_filename = modified_config_file.name
            modified_config_file.close()
        except PermissionError as e:
            print(f"ERROR: Permission denied creating temporary config file: {e}", file=sys.stderr)
            sys.exit(1)
        except yaml.YAMLError as e:
            print(f"ERROR: Failed to serialize config to YAML: {e}", file=sys.stderr)
            sys.exit(1)
        except Exception as e:
            print(f"ERROR: Failed to create modified config file: {e}", file=sys.stderr)
            sys.exit(1)

        # Run FTW tests
        print("\n" + "="*60)
        print("Running FTW tests...")
        print("="*60 + "\n")

        # Get the directory where this script is located
        script_dir = os.path.dirname(os.path.abspath(__file__))

        ftw_cmd = [
            "go", "run",
            f"-modfile={script_dir}/go.mod",
            "github.com/coreruleset/go-ftw/v2",
            "run",
            "-d", args.rules_directory,
            "--config", modified_config_filename,
            "--log-file", log_filename,
            "--read-timeout", "10s"
        ]

        if args.output_log:
            ftw_cmd += ["-f", args.output_log]
        
        if args.output_format:
            ftw_cmd += ["--output", args.output_format]

        print(f"Configuration:")
        print(f"  Target: {target_host}:{target_port}")
        print(f"  Log file: {log_filename}")
        print(f"  Modified config: {modified_config_filename}\n")
        print(f"Executing: {' '.join(ftw_cmd)}\n")

        ftw_result = subprocess.run(ftw_cmd)

        print(f"\n" + "="*60)
        print(f"FTW tests completed with exit code: {ftw_result.returncode}")
        print(f"Logs saved to: {log_filename}")
        print("="*60)

        # Clean up modified config file
        try:
            os.unlink(modified_config_filename)
        except Exception:
            pass

        sys.exit(ftw_result.returncode)

    finally:
        # Cleanup: stop port-forward if it was started
        if port_forward_process:
            print("\nStopping port-forward...")
            port_forward_process.terminate()
            try:
                port_forward_process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                print("Port-forward did not stop gracefully, killing...")
                port_forward_process.kill()
                port_forward_process.wait()


if __name__ == "__main__":
    main()
