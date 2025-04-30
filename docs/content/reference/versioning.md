# Releases and Versions

[Clastix Labs](https://github.com/clastix) organization publishes Kamaji's versions that correspond to specific project milestones and sets of new features.
These versions are available in different types of release artifacts.

## Types of release artifacts

### Latest Releases

CI is responsible for building OCI and Helm Chart for every commit in the main branch (`master`):
The latest artifacts are aimed for rapid development tests and evaluation process.

Usage of the said artefacts is not suggested for production use-case due to missing version pinning of artefacts:

- `latest` for OCI image (e.g.: `docker.io/clastix/kamaji:latest`)
- `0.0.0+latest` for the Helm Chart managed by CLASTIX (`https://clastix.github.io/charts`)

### Edge Releases

Edge Release artifacts are published on a monthly basis as part of the open source project.
Versioning follows the form `edge-{year}.{month}.{incremental}` where incremental refers to the monthly release.
For example, `edge-24.7.1` is the first edge release shipped in July 2024.
The full list of edge release artifacts can be found on the Kamaji's GitHub [releases page](https://github.com/clastix/kamaji/releases).

Edge Release artifacts contain the code in from the main branch at the point in time when they were cut.
This means they always have the latest features and fixes, and have undergone automated testing as well as maintainer code review.
Edge Releases may involve partial features that are later modified or backed out.
They may also involve breaking changes, of course, we do our best to avoid this.

Edge Releases are generally considered production ready and the project will mark specific releases as _"not recommended"_ if bugs are discovered after release.

| Kamaji      | Management Cluster | Tenant Cluster       |
|-------------|--------------------|----------------------|
| edge-25.4.1 | v1.22+             | [v1.30.0 .. v1.33.0] |


Using Edge Release artifacts and reporting bugs helps us ensure a rapid pace of development and is a great way to help maintainers.
We publish edge release guidance as part of the release notes and strive to always provide production-ready artifacts.

### Stable Releases

As of July 2024, [Clastix Labs](https://github.com/clastix) does no longer provide release artifacts following its own semantic versioning:
this choice has been put in place to help monetize CLASTIX in the development and maintenance of the Kamaji project.

Stable artifacts such as OCI (containers) and Helm Chart ones are available on a subscription basis maintained by [CLASTIX](https://clastix.io):
learn more about the available [Subscription Plans](https://clastix.io/support/).
