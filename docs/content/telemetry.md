# Telemetry

This document outlines the telemetry feature within the Kamaji project, detailing the rationale behind data collection, the nature of the data collected, data handling practices, and instructions for opting out.

## Why We Collect Telemetry

The Kamaji project, being open source, benefits from insights into how it is used. These insights help the project maintainers make informed decisions regarding feature prioritization, test automation, and bug fixes. Without this data, decisions on feature deprecation and enhancements would be based on limited information, potentially hindering the project's evolution and maintainability. Our goal is to ensure Kamaji's development is driven by the needs of its community, and telemetry data plays a crucial role in achieving this.

## What We Collect and How

It's important to clarify that our interest lies in the usage patterns of Kamaji, not in personal information about its users. We collect data about Kamaji version and Tenant Control Planes.

### Telemetry Payload Example

Below is a simplified example of what a telemetry payload might look like:

```json
// General status
{"uuid": "56279633-3131-436b-b8f2-9008a49a2f12", "running": 1, "sleeping": 0, "not_ready": 0, "upgrading": 0, "kamaji_version": "v0.6.1", "kubernetes_version": "v1.27.3"}
```

```json
// Creating a TCP
{"tcp_version": "v1.26.0", "kamaji_version": "v0.6.1", "kubernetes_version": "v1.27.3"}
```

```json
// Modifying a TCP (n.b.: version upgrade)
{"kamaji_version": "v0.6.1", "new_tcp_version": "v1.27.0", "old_tcp_version": "v1.26.0", "kubernetes_version": "v1.27.3"}
```

```json
// Deleting a TCP
{"status": "Ready", "tcp_version": "v1.27.0", "kamaji_version": "v0.6.1", "kubernetes_version": "v1.27.3"}
```

Data is collected through a component within Kamaji, which periodically sends this information to our backend. The telemetry backend, managed by the Kamaji maintainers, ensures data is stored securely and access is strictly controlled.

## Telemetry Opt-Out
We respect the privacy and autonomy of our users. If you prefer not to participate in telemetry, Kamaji provides an easy opt-out mechanism.

For Helm based deployments, include the `--set telemetry.disabled=true` flag when installing or upgrading Kamaji.

For manual deployments, set the flag `--disable-telemetry=true` int the Kamaji controller configuration to disable telemetry reporting.

To re-enable telemetry, simply reverse the opt-out process using the appropriate method for your deployment.

## Conclusion
Telemetry in Kamaji is designed to foster a data-driven development process that aligns with the needs and preferences of our user community. Your participation helps us make Kamaji better for everyone.
