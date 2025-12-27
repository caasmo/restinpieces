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

In this model, Litestream is imported as a library and runs in a background goroutine within the main application process.

**Advantages:**
-   **Self-Contained Architecture:** Each application is a truly self-contained unit, aligning with the project's core philosophy.
-   **Simplified Security:** No cross-application file access is needed, avoiding the complexity of shared group permissions.
-   **Reliability:** The application and its replication process share the same lifecycle and fail/restart together.
-   **Simplified Operations:** No separate daemon needs to be installed, configured, or monitored.
-   **Systemd Integration:** Works seamlessly with the existing hardened `systemd` configuration.

**Disadvantages:**
-   **Larger App Binaries:** Including the Litestream library increases the size of each application binary.
-   **Decentralized Management:** Replication status, S3 credentials, and Litestream versions are managed on a per-application basis.

##### Centralized Litestream Daemon

In this model, a single, system-wide `litestream` process runs as a separate daemon and is configured to manage replication for all applications.

**Advantages:**
-   **Slimmer App Binaries:** Application binaries are smaller as they do not need to include the Litestream library.
-   **Centralized Management:** Provides a single place to manage S3 configuration, monitor replication status, and standardize the Litestream version across all apps.
-   **Lower Memory Footprint:** A single daemon process consumes less memory than multiple embedded instances.

**Disadvantages:**
-   **Violates Self-Contained Architecture:** Introduces a shared, external dependency, moving away from the "one process" philosophy.
-   **Increased Security Complexity:** Requires the central daemon to have read access to all application databases, which typically involves relaxing file permissions with shared groups.
-   **Decoupled Lifecycle:** The replication process can fail independently of the applications, potentially leading to unmonitored backup failures.
-   **Increased Operational Overhead:** Requires managing and monitoring an additional system-wide daemon.

