# Welcome

Welcome to Arktos! 

-   [Before you get started](#before-you-get-started)
    -   [Code of Conduct](#code-of-conduct)
-   [Getting started](#getting-started)
-   [Your First Contribution](#your-first-contribution)
    -   [Find something to work on](#find-something-to-work-on)
        -   [Find a good first topic](#find-a-good-first-topic)
        -   [Work on an Issue](#work-on-an-issue)
        -   [File an Issue](#file-an-issue)
-   [Contributor Workflow](#contributor-workflow)
    -   [Creating Pull Requests](#creating-pull-requests)
    -   [Code Review](#code-review)
    -   [Testing](#testing)
    -   [Documents and Support](#documents-and-support)

# Before you get started

## Code of Conduct

Please make sure to read and observe our [Code of Conduct](https://github.com/CentaurusInfra/arktos/blob/master/code-of-conduct.md).

# Getting started

- Fork the repository on GitHub
- Visit [README](https://github.com/CentaurusInfra/arktos#build-arktos) for build instructions.


# Your First Contribution

We will help you to contribute in different areas like filing issues, developing features, fixing critical bugs and getting your work reviewed and merged.

If you have questions about the development process, feel free to jump into our [slack channel](https://app.slack.com/client/TMNECBVT5/CRRUU7137) or join our [email group](https://groups.google.com/forum/#!forum/arktos-user).

## Find something to work on

We are always in need of help, be it fixing documentation, reporting bugs or writing some code.
Look at places where you feel best coding practices aren't followed, code refactoring is needed or tests are missing.
Here is how you get started.

### Find a good first topic

There are multiple repositories ([Arktos](https://github.com/CentaurusInfra/arktos), [Arktos-vm-runtime](https://github.com/CentaurusInfra/arktos-vm-runtime), [Arktos-cniplugins](https://github.com/CentaurusInfra/arktos-cniplugins)) within the Arktos organization.
Each repository has beginner-friendly issues that provide a good starting point.
For example, [Arktos](https://github.com/CentaurusInfra/arktos) has [help wanted](https://github.com/CentaurusInfra/arktos/labels/help%20wanted) and [good first issue](https://github.com/CentaurusInfra/arktos/labels/good%20first%20issue) labels for issues that should not need deep knowledge of the system. We can help new contributors who wish to work on such issues.


### Work on an issue

When you are willing to take on an issue, you can assign it to yourself. Just reply with `/assign` or `/assign @yourself` on an issue,
then the robot will assign the issue to you and your name will present at `Assignees` list.

### File an Issue

While we encourage everyone to contribute code, it is also appreciated when someone reports an issue.
Issues should be filed under the appropriate Arktos sub-repository.

*Example:* An Arktos issue should be opened to [Arktos](https://github.com/CentaurusInfra/arktos). And the same should apply to [Arktos-vm-runtime](https://github.com/CentaurusInfra/arktos-vm-runtime) and [Arktos-cniplugins](https://github.com/CentaurusInfra/arktos-cniplugins).

Please follow the prompted submission guidelines while opening an issue.

# Contributor Workflow

Please do not ever hesitate to ask a question or send a pull request.

This is a rough outline of what a contributor's workflow looks like:

- Fork [Arktos](https://github.com/CentaurusInfra/arktos).
- Create a topic branch in your forked repo from where to base the contribution. This is usually master.
- Make commits of logical units.
- Make sure commit messages are in the proper format (see below).
- Push changes in a topic branch to a personal fork of the repository.
- Submit a pull request to [Arktos](https://github.com/CentaurusInfra/arktos).
- The PR must receive an approval from two team members including at least one maintainer.

## Creating Pull Requests

Pull requests are often called simply "PR".
Arktos generally follows the standard [github pull request](https://help.github.com/articles/about-pull-requests/) process.
To submit a proposed change, please develop the code/fix and add new test cases.
After that, run these local verifications before submitting pull request to predict the pass or
fail of continuous integration.

## Code Review

To make it easier for your PR to receive reviews, consider the reviewers will need you to:

* follow [good coding guidelines](https://github.com/golang/go/wiki/CodeReviewComments).
* write [good commit messages](https://chris.beams.io/posts/git-commit/).
* break large changes into a logical series of smaller patches which individually make easily understandable changes, and in aggregate solve a broader issue.
* label PRs with appropriate reviewers: to do this read the messages the bot sends you to guide you through the PR process.

### Format of the commit message

We follow a rough convention for commit messages that is designed to answer two questions: what changed and why.
The subject line should feature the what and the body of the commit should describe the why.

```
scripts: add test codes for metamanager

this add some unit test codes to imporve code coverage for metamanager

Fixes #12
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

There are multiple types of tests.
The location of the test code varies with type, as do the specifics of the environment needed to successfully run the test:

* Unit: These confirm that a particular function behaves as intended. Unit test source code can be found adjacent to the corresponding source code within a given package. These are easily run locally by any developer.
* Integration: These tests cover interactions of package components or interactions between Arktos components. 
* End-to-end ("e2e"): These are broad tests of overall system behavior and coherence. 

Continuous integration will run these tests on PRs.

## Documents and Support

The [design document folder](https://github.com/CentaurusInfra/arktos/tree/master/docs/design-proposals) contains the detailed design of already implemented features, and also some thoughts for planned features.

The [user guide folder](https://github.com/CentaurusInfra/arktos/tree/master/docs/user-guide) provides information about these features from users' perspective.

To report a problem, please [create an issue](https://github.com/CentaurusInfra/arktos/issues) in the project repo. 

To ask a question, you can either chat with project members in the [slack channel](https://app.slack.com/client/TMNECBVT5/CRRUU7137), post in the [email group](https://groups.google.com/forum/#!forum/arktos-user), or [create an issue](https://github.com/CentaurusInfra/arktos/issues) of question type in the repo.
