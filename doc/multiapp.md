# Multi-App Deployment Strategy

This document outlines the strategy for deploying multiple, independent RestInPieces applications onto a single Virtual Machine (VM). The core principle is to use battle-tested Linux security primitives and `systemd` for process isolation, providing a secure, simple, and high-performance alternative to containerization for this specific use case.

Our approach focuses on:
*   **User-level Isolation:** Each application runs as its own dedicated Unix user.
*   **Filesystem Security:** Strict file permissions and `systemd` sandboxing prevent applications from accessing each other's data.
*   **Simplicity and Low Overhead:** Avoids the complexity of container orchestration (like Docker or Kubernetes) which is not required for this architecture.

## Systemd Hardening

Our `systemd` setup provides robust security and isolation, leveraging the same underlying Linux kernel features that containers use.

### Your Current Setup is 90% There

The `ripdep` tool already configures each service with a strong security baseline:
*   ✅ User/group isolation per app (`User=`, `Group=`)
*   ✅ Filesystem sandboxing (`ProtectSystem=full`, `ProtectHome=read-only`, `PrivateTmp=true`)
*   ✅ Privilege control (`NoNewPrivileges=true`)
*   ✅ Capability restrictions (dropping all but `CAP_NET_BIND_SERVICE`)

This is a genuinely secure foundation. If one application is compromised, the attacker's capabilities are severely limited:
*   They **cannot** read other applications' data due to Unix user permissions.
*   They **cannot** escalate privileges on the system.
*   They have **limited** access to the host filesystem.

### Systemd vs. Containers

For a solo developer or small team running self-contained Go applications on a single server, `systemd` is often simpler, more transparent, and just as secure as a container-based setup.

Both `systemd` and containers (like Docker) utilize the exact same Linux kernel security features:
*   Namespaces (PID, mount, network, etc.)
*   Cgroups (for resource limiting)
*   Capabilities (for dropping root privileges)
*   Seccomp (for syscall filtering)
*   SELinux/AppArmor (for mandatory access control)

The narrative that "real production requires containers" is primarily driven by use cases that do not apply here, such as managing complex microservices architectures or applications with heavy dependency trees (e.g., Python/Node.js).

**Conclusion:** Stick with the hardened `systemd` approach. You are using kernel isolation primitives directly, achieving strong security without the overhead and complexity of a container runtime.

## Prometheus Monitoring

For monitoring multiple applications on the same VM, each application can expose its own `/metrics` endpoint on a different port. A single Prometheus instance (running on the host or elsewhere) can then be configured to scrape all of these endpoints.

Example `prometheus.yml` scrape configuration:
```yaml
scrape_configs:
  - job_name: 'restinpieces-apps'
    static_configs:
      - targets:
          - 'localhost:9091'  # App 1
          - 'localhost:9092'  # App 2
          - 'localhost:9093'  # App 3
```
This allows you to aggregate metrics from all applications into a single dashboard while keeping the applications themselves completely separate.

## Litestream (Database Backups)

The `ripdep` tool configures Litestream based on a per-application configuration file. To manage backups for multiple applications on the same host, each application's `litestream.yml` must specify a unique database path and, critically, a unique remote replica URL (e.g., a different sub-path in an S3 bucket).

**Example for App 1 (`app1/litestream.yml`):**
```yaml
dbs:
  - path: /home/app1/data/app.db
    replicas:
      - type: s3
        bucket: my-app-backups
        path: app1/db
```

**Example for App 2 (`app2/litestream.yml`):**
```yaml
dbs:
  - path: /home/app2/data/app.db
    replicas:
      - type: s3
        bucket: my-app-backups
        path: app2/db
```
This ensures that each application's database is backed up to its own isolated location, preventing data from being overwritten.
