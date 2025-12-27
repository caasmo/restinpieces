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

### Accessing Metrics Remotely
If your Prometheus scraper is running on a different machine, or if you want to view an application's metrics endpoint from your local browser, you can use an SSH tunnel to securely forward the port.

```bash
# From your local machine, forward the server's port 9091 to your local port 9091
ssh -L 9091:localhost:9091 user@your-server.com
```
You can now browse to `http://localhost:9091/metrics` on your local machine to view the metrics. The same technique can be used to configure a remote Prometheus instance to scrape the tunneled port.

## Backups

When running multiple applications, you have two primary strategies for database backups, which can be used independently or together for a defense-in-depth approach.

### 1. Local Backups

The framework has a built-in job to perform periodic local backups of the SQLite database. In a multi-app environment, each application manages its own backup schedule independently. This requires no special configuration beyond what is already handled within each application's settings. It is simple, robust, and provides a baseline of data safety.

### 2. Real-Time Replication (Litestream)

For continuous, real-time replication to offsite storage like Amazon S3, the framework integrates with Litestream. In a multi-app context, you can run Litestream in two ways: embedded within each application service or as a single, centralized daemon.

**Our strong recommendation is to use the embedded Litestream model.** This approach aligns best with the framework's philosophy of creating simple, secure, and self-contained services.

#### Comparison: Embedded vs. Centralized

##### Embedded Litestream (Recommended)

Each application's `systemd` service runs its own `litestream` process alongside the application binary.

| Pros                               | Cons                                       |
| ---------------------------------- | ------------------------------------------ |
| ✅ **True Isolation**                | Marginally higher memory usage             |
| ✅ **Enhanced Reliability**          | S3 credentials configured per-application  |
| ✅ **Operational Simplicity**        |                                            |
| ✅ **Superior Security**             |                                            |

-   **Isolation & Security**: This is the cleanest approach. Since each application runs its own replication process under its own user, no cross-app file access is needed. You don't need to create shared Unix groups or relax file permissions, preserving the strict user-level isolation.
-   **Reliability**: The application and its backup process share the same lifecycle. If the app restarts, its Litestream process restarts with it, ensuring replication is always running when the app is.
-   **Simplicity**: There is no separate daemon to manage. `ripdep` already configures this out of the box, fitting perfectly with the hardened `systemd` setup.

##### Centralized Litestream Daemon

A single, system-wide `litestream` service monitors and replicates all application databases.

| Pros                               | Cons                                       |
| ---------------------------------- | ------------------------------------------ |
| Centralized monitoring & logging   | ❌ **Increased Complexity**                |
| Single place for S3 credentials    | ❌ **Weakened Security**                   |
| Slightly lower memory footprint    | ❌ **Reduced Reliability**                   |

-   **Complexity & Security**: While it offers central management, a separate daemon is more complex to maintain. More importantly, it requires weakening the security model. The central `litestream` user would need read access to every application's private database files, typically forcing the use of shared groups and breaking the "one user, one app" isolation principle.
-   **Reliability**: Because the backup process is decoupled, a failure in the central daemon could silently halt backups for all applications.

**Conclusion:** For an architecture focused on security and simplicity, the **embedded Litestream model is the clear winner.** The small cost in memory is a worthwhile trade-off for the significant gains in isolation, reliability, and operational simplicity.
