# kubeterra
Simple Kubernetes Terraform integration - Manage your Terraform environment with
Kubernetes!

## About & Motivation

Kubeterra was created out of necessity for automation of managing cloud
resources in on-demand manner, where configurations are defined as terraform
modules.

Kubeterra itself is a controller manager that run on kubernetes and operating on
CustomResources.

## Installation

We strongly recommend that you use an [official release][3] of kubeterra. The tarballs for each release contain the
command-line client **and** version-specific sample YAML files for deploying kubeterra to your cluster.
Follow the instructions under the **Install** section of [our documentation][21] to get started.

_The code and sample YAML files in the master branch of the kubeterra repository are under active development and are not guaranteed to be stable. Use them at your own risk!_

## More information

[The documentation][21] provides a getting started guide, plus information about building from source, architecture, extending kubeterra, and more.

Please use the version selector at the top of the site to ensure you are using the appropriate documentation for your version of kubeterra.

## Troubleshooting

If you encounter issues [file an issue][1] or talk to us on the [#kubeterra channel][12] on the [loodse Slack][15].

## Contributing

Thanks for taking the time to join our community and start contributing!

Feedback and discussion are available on [the mailing list][11].

### Before you start

* Please familiarize yourself with the [Code of Conduct][4] before contributing.
* See [CONTRIBUTING.md][2] for instructions on the developer certificate of origin that we require.
* Read how [we're using ZenHub][13] for project and roadmap planning

### Pull requests

* We welcome pull requests. Feel free to dig through the [issues][1] and jump in.

## Changelog

See [the list of releases][3] to find out about feature changes.

[1]: https://github.com/loodse/kubeterra/issues
[2]: https://github.com/loodse/kubeterra/blob/master/CONTRIBUTING.md
[3]: https://github.com/loodse/kubeterra/releases
[4]: https://github.com/loodse/kubeterra/blob/master/CODE_OF_CONDUCT.md

[11]: https://groups.google.com/forum/#!forum/projectkubeterra
[12]: https://loodse.slack.com/messages/kubeterra
[13]: https://github.com/loodse/kubeterra/blob/master/Zenhub.md
[15]: http://slack.loodse.io/

[21]: https://github.com/loodse/kubeterra/tree/master/docs