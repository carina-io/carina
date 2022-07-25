# Contributing Giude

- [Contributing Giude](#contributing-giude)
- [Before you get started](#before-you-get-started)
  - [Code of Conduct](#code-of-conduct)
  - [Community Expectations](#community-expectations)
- [Getting started](#getting-started)
- [Your First Contribution](#your-first-contribution)
  - [Find something to work on](#find-something-to-work-on)
    - [Find a good first topic](#find-a-good-first-topic)
      - [Work on an issue](#work-on-an-issue)
    - [File an Issue](#file-an-issue)
- [Contributor Workflow](#contributor-workflow)
  - [Creating Pull Requests](#creating-pull-requests)
  - [Code Review](#code-review)
    - [Format of the commit message](#format-of-the-commit-message)
  - [Testing](#testing)

# Before you get started

## Code of Conduct

Please make sure to read and observe our [Code of Conduct](https://github.com/carina-io/carina/blob/main/CODE_OF_CONDUCT.md).

## Community Expectations

Carina is a community project driven by its community which strives to promote a healthy, friendly and productive environment.
Carina is a standard kubernetes CSI plugin. Users can use standard kubernetes storage resources like storageclass/PVC/PV to request storage media.

# Getting started

- Fork the repository on GitHub.
- Make your changes on your fork repository.
- Submit a PR.


# Your First Contribution

We will help you to contribute in different areas like filing issues, developing features, fixing critical bugs and
getting your work reviewed and merged.

If you have questions about the development process,
feel free to [file an issue](https://github.com/carina-io/carina/issues/new/choose).

## Find something to work on

We are always in need of help, be it fixing documentation, reporting bugs or writing some code.
Look at places where you feel best coding practices aren't followed, code refactoring is needed or tests are missing.
Here is how you get started.

### Find a good first topic

There are [multiple repositories](https://github.com/carina-io) within the Carina organization.
Each repository has beginner-friendly issues that provide a good first issue.
For example, [carina-io/carina](https://github.com/carina-io/carina) has
[help wanted](https://github.com/carina-io/carina/labels/help%20wanted) and
[good first issue](https://github.com/carina-io/carina/labels/good%20first%20issue)
labels for issues that should not need deep knowledge of the system.
We can help new contributors who wish to work on such issues.

Another good way to contribute is to find a documentation improvement, such as a missing/broken link.
Please see [Contributing](#contributing) below for the workflow.

#### Work on an issue

When you are willing to take on an issue, just reply on the issue. The maintainer will assign it to you.

### File an Issue

While we encourage everyone to contribute code, it is also appreciated when someone reports an issue.
Issues should be filed under the appropriate Carina sub-repository.

*Example:* a Carina issue should be opened to [carina-io/carina](https://github.com/carina-io/carina/issues).

Please follow the prompted submission guidelines while opening an issue.

# Contributor Workflow

Please do not ever hesitate to ask a question or send a pull request.

This is a rough outline of what a contributor's workflow looks like:

- Create a topic branch from where to base the contribution. This is usually main.
- Make commits of logical units.
- Push changes in a topic branch to a personal fork of the repository.
- Submit a pull request to [carina-io/carina](https://github.com/carina-io/carina).

## Creating Pull Requests

Pull requests are often called simply "PR".
Carina generally follows the standard [github pull request](https://help.github.com/articles/about-pull-requests/) process.
To submit a proposed change, please develop the code/fix and add new test cases.
After that, run these local verifications before submitting pull request to predict the pass or
fail of continuous integration.

* Run and pass `make vet`
* Run and pass `make test`

## Code Review

To make it easier for your PR to receive reviews, consider the reviewers will need you to:

* follow [good coding guidelines](https://github.com/golang/go/wiki/CodeReviewComments).
* write [good commit messages](https://chris.beams.io/posts/git-commit/).
* break large changes into a logical series of smaller patches which individually make easily understandable changes, and in aggregate solve a broader issue.

### Format of the commit message

We follow a rough convention for commit messages that is designed to answer two questions: what changed and why. The subject line should feature the what and the body of the commit should describe the why.

```
carina-node: add test codes for carina-node

this add some unit test codes to improve code coverage for carina-node

Fixes #666
```

The format can be described more formally as follows:

```
<subsystem>: <what changed>
<BLANK LINE>
<why this change was made>
<BLANK LINE>
<footer>
```

The first line is the subject and should be no longer than 70 characters, the second line is always blank, and other lines should be wrapped at 80 characters. This allows the message to be easier to read on GitHub as well as in various git tools.

Note: if your pull request isn't getting enough attention, you can use the reach out on WechatGroup to get help finding reviewers.

## Testing

There are multiple types of tests. The location of the test code varies with type, as do the specifics of the environment needed to successfully run the test:

- Unit: These confirm that a particular function behaves as intended. Unit test source code can be found adjacent to the corresponding source code within a given package. These are easily run locally by any developer.
- Integration: These tests cover interactions of package components or interactions between components and Kubernetes control plane components like API server. 
- End-to-end ("e2e"): These are broad tests of overall system behavior and coherence. 

Continuous integration will run these tests on PRs.