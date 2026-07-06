# -*- coding: utf-8 -*-
#!/usr/bin/env python3
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request


LOG_FILE = "/data/boot3/logs/sahara-social/catalina.out"
NOTIFY_SCRIPT = "/prosh/salt-agent/notify/notify_teams_webhooks.py"
NOTIFY_CONFIG = "/prosh/salt-agent/notify/settings.json"
YELLOW = "\033[33m"
RESET = "\033[0m"


def run_command(args, check=False, capture_output=True, cwd=None):
    return subprocess.run(
        args,
        cwd=cwd,
        text=True,
        capture_output=capture_output,
        check=check,
    )


def print_matching_containers(module_name):
    result = run_command(["docker", "ps", "-a"], capture_output=True)
    if result.returncode != 0:
        print("WARNING: docker ps -a failed:")
        print((result.stderr or result.stdout).strip())
        return

    print(f"[INFO] docker ps -a | grep {module_name}")
    matched = False
    for line in result.stdout.splitlines():
        if module_name in line:
            print(line)
            matched = True
    if not matched:
        print(f"[INFO] No docker ps -a rows matched: {module_name}")


def stop_container(module_name):
    print(f"[1/2] Stopping container {module_name}...")
    result = run_command(["docker", "stop", module_name, "-t", "90"])
    if result.returncode != 0:
        print(
            f"WARNING: container {module_name} may not be running or stop failed. Continuing..."
        )


def start_container(module_name):
    print(f"[2/2] Starting container {module_name}...")
    result = run_command(["docker", "start", module_name])
    output = (result.stdout or "") + (result.stderr or "")

    if result.returncode == 0:
        print(f"Container {module_name} started successfully.")
        return True

    if "attaching to network failed" not in output:
        print(f"ERROR: failed to start container {module_name}.")
        print(f"Error output: {output.strip()}")
        return False

    print("Detected network attach failure. Trying to reconnect the container network...")
    time.sleep(3)
    network_name = get_first_network_name(module_name)
    if not network_name:
        print(f"ERROR: unable to find network name for container {module_name}.")
        print(f"Original error: {output.strip()}")
        return False

    print(f"Detected network name: {network_name}")
    run_command(["docker", "network", "disconnect", network_name, module_name])
    time.sleep(1)

    print(f"Running: docker network connect {network_name} {module_name}")
    connect_result = run_command(["docker", "network", "connect", network_name, module_name])
    if connect_result.returncode != 0:
        print(
            f"ERROR: unable to connect container {module_name} to network {network_name}."
        )
        print(f"Original error: {output.strip()}")
        return False

    print("Network connected successfully. Starting container again...")
    retry_result = run_command(["docker", "start", module_name], capture_output=False)
    if retry_result.returncode != 0:
        print(f"ERROR: container {module_name} failed to start after network reconnect.")
        return False

    print(f"Container {module_name} started successfully.")
    return True


def get_first_network_name(module_name):
    payload = docker_inspect(module_name)
    if not payload:
        return None
    networks = (
        payload[0]
        .get("NetworkSettings", {})
        .get("Networks", {})
    )
    return next(iter(networks.keys()), None)


def docker_inspect(module_name):
    result = run_command(["docker", "inspect", module_name])
    if result.returncode != 0:
        return None
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError:
        return None


def extract_8080_host_port(inspect_payload):
    if not inspect_payload:
        return None

    ports = inspect_payload[0].get("NetworkSettings", {}).get("Ports", {})
    bindings = ports.get("8080/tcp") or []
    for binding in bindings:
        host_port = binding.get("HostPort")
        if host_port:
            return host_port
    return None


def get_health_status(url):
    request = urllib.request.Request(url, method="GET")
    try:
        with urllib.request.urlopen(request, timeout=5) as response:
            return response.getcode()
    except urllib.error.HTTPError as exc:
        return exc.code
    except (urllib.error.URLError, TimeoutError, OSError):
        return None


def tail_log_lines(lines=20):
    result = run_command(["tail", "-n", str(lines), LOG_FILE])
    output = (result.stdout or "") + (result.stderr or "")
    return output.rstrip()


def wait_for_healthy(
    health_url,
    timeout_seconds=120,
    interval_seconds=5,
    health_getter=get_health_status,
    sleep=time.sleep,
    tail_log=tail_log_lines,
    out=sys.stdout,
):
    print("重启完成后，等待应用程序启动成功，心跳监测中...", file=out)
    deadline = time.monotonic() + timeout_seconds

    while time.monotonic() <= deadline:
        status_code = health_getter(health_url)
        if status_code == 200:
            print(f"[INFO] Health check passed: {health_url} returned 200.", file=out)
            print(f"[INFO] tail -n 20 {LOG_FILE}", file=out)
            log_output = tail_log()
            if log_output:
                print(log_output, file=out)
            return True

        if status_code is None:
            print("[INFO] Health check pending...", file=out)
        else:
            print(f"[INFO] Health check returned HTTP {status_code}. Waiting...", file=out)
        sleep(interval_seconds)

    print(f"[INFO] tail -n 20 {LOG_FILE}", file=out)
    log_output = tail_log()
    if log_output:
        print(log_output, file=out)
    print(
        YELLOW
        + "[WARNING]Startup health check is unknown. Please log in to the server and "
        + "check the application startup status manually. Suggested command: "
        + f"tail -fn 300 {LOG_FILE}"
        + RESET,
        file=out,
    )
    return False


def notify_teams(module_name):
    if not (os.path.isfile(NOTIFY_SCRIPT) and os.path.isfile(NOTIFY_CONFIG)):
        print("Skipping Teams notification: notify script or config does not exist.")
        return

    print("Sending update notification to Teams...")
    result = run_command(
        [
            "python3",
            NOTIFY_SCRIPT,
            "--modules",
            module_name,
            "--show-all-containers",
            "--config",
            NOTIFY_CONFIG,
        ],
        cwd="/prosh/salt-agent/",
        capture_output=False,
    )
    if result.returncode == 0:
        print("Teams notification sent.")
    else:
        print(f"WARNING: Teams notification failed with exit code {result.returncode}.")


def usage(script_name):
    print(f"Usage: {script_name} <module_name>")
    print("")
    print("Examples:")
    print(f"  {script_name} frank")
    print(f"  {script_name} example-frank")


def main(argv):
    if len(argv) < 2:
        usage(argv[0])
        return 1

    module_name = argv[1]
    print("=========================================")
    print(f"Starting restart for Docker container: {module_name}")
    print("=========================================")

    print_matching_containers(module_name)
    stop_container(module_name)
    if not start_container(module_name):
        return 1

    print("")
    print("=========================================")
    print(f"Container {module_name} restart completed.")
    print("=========================================")

    inspect_payload = docker_inspect(module_name)
    host_port = extract_8080_host_port(inspect_payload)
    if not host_port:
        print(f"ERROR: unable to find host port mapped to container port 8080 for {module_name}.")
        print(f"[INFO] tail -n 20 {LOG_FILE}")
        log_output = tail_log_lines()
        if log_output:
            print(log_output)
        return 1

    health_url = f"http://localhost:{host_port}/actuator/health"
    healthy = wait_for_healthy(health_url)
    if healthy:
        notify_teams(module_name)
        return 0

    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv))
