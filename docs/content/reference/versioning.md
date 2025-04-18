# Releases and Versions

[Clastix Labs](https://github.com/clastix) organization publishes Kamaji's versions that correspond to specific project milestones and sets of new features. These versions are available in different types of release artifacts.

## Types of release artifacts

### Edge Releases

Edge Release artifacts are published on a monthly basis as part of the open source project. Versioning follows the form `edge-{year}.{month}.{incremental}` where incremental refers to the monthly release. For example, `edge-24.7.1` is the first edge release shipped in July 2024. The full list of edge release artifacts can be found on the Kamaji's GitHub [releases page](https://github.com/clastix/kamaji/releases).

Edge Release artifacts contain the code in from the main branch at the point in time when they were cut. This means they always have the latest features and fixes, and have undergone automated testing as well as maintainer code review. Edge Releases may involve partial features that are later modified or backed out. They may also involve breaking changes, of course, we do our best to avoid this. Edge Releases are generally considered production ready, and the project will mark specific releases as “_not recommended_” if bugs are discovered after release.

| Kamaji       | Management Cluster | Tenant Cluster       |
|--------------|--------------------|----------------------|
| edge-24.12.1 | v1.22+             | [v1.29.0 .. v1.31.4] |


Using Edge Release artifacts and reporting bugs helps us ensure a rapid pace of development and is a great way to help maintainers. We publish edge release guidance as part of the release notes and strive to always provide production-ready artifacts.

### Stable Releases

Stable Release artifacts of Kamaji follow semantic versioning, whereby changes in major version denote large feature additions and possible breaking changes and changes in minor versions denote safe upgrades without breaking changes. As of July 2024, [Clastix Labs](https://github.com/clastix) does no longer provide stable release artifacts.

Stable Release artifacts are offered now on a subscription basis by [CLASTIX](https://clastix.io), the main Kamaji project contributor. Learn more about the available [Subscription Plans](https://clastix.io/support/).
