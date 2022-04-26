# Contributing Giude

- Before you get started
  - Code of Conduct
- Getting started
- Contributor Workflow
  - Creating Pull Requests
  - Code Review
  - Testing

# Before you get started

## Code of Conduct

Please make sure to read and observe our [Code of Conduct](https://github.com/carina-io/carina/blob/main/CODE_OF_CONDUCT.md).

# Getting started

- Fork the repository on GitHub
- Read the [docs](https://github.com/carina-io/carina/tree/main/docs) for deployment.

# Your First Contribution

We will help you to contribute in different areas like filing issues, developing features, fixing bugs and getting your work reviewed and merged.

# Contributor Workflow

Please do not ever hesitate to ask a question or send a pull request.

This is a rough outline of what a contributor's workflow looks like:

- Create a topic branch from where to base the contribution. This is usually master.
- Make commits of logical units.
- Make sure commit messages are in the proper format (see below).
- Push changes in a topic branch to a personal fork of the repository.
- Submit a pull request
- The PR must receive an approval from maintainers.

## Creating Pull Requests

Fabedge generally follows the standard [github pull request](https://help.github.com/articles/about-pull-requests/) process. 

## Code Review

To make it easier for your PR to receive reviews, consider the reviewers will need you to:

- follow [good coding guidelines](https://github.com/golang/go/wiki/CodeReviewComments).
- write [good commit messages](https://chris.beams.io/posts/git-commit/).
- break large changes into a logical series of smaller patches which individually make easily understandable changes, and in aggregate solve a broader issue.

### Format of the commit message

We follow a rough convention for commit messages that is designed to answer two questions: what changed and why. The subject line should feature the what and the body of the commit should describe the why.

```
agent: add test codes for manager

this add some unit test codes to improve code coverage for agent

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

Note: if your pull request isn't getting enough attention, you can use the reach out on Slack to get help finding reviewers.

## Testing

There are multiple types of tests. The location of the test code varies with type, as do the specifics of the environment needed to successfully run the test:

- Unit: These confirm that a particular function behaves as intended. Unit test source code can be found adjacent to the corresponding source code within a given package. These are easily run locally by any developer.
- Integration: These tests cover interactions of package components or interactions between components and Kubernetes control plane components like API server. 
- End-to-end ("e2e"): These are broad tests of overall system behavior and coherence. 

Continuous integration will run these tests on PRs.