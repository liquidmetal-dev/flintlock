# flintlock :heart:s your contributions

Thank you for taking the time to contribute to `flintlock`.

We gratefully welcome improvements to all areas; from code, to documentation;
from bug reports to feature design.

This guide should cover all aspects of how to interact with the project
and how to get involved in development as smoothly as possible.

If we have missed anything you think should be included, or if anything is not
clear, we also accept contributions to this contribution doc :smile:.

The list of maintainers can be found [here](MAINTAINERS). To reach out to the team
please join the [#liquid-metal][slack-channel] slack channel.

Reading docs is often tedious, so we'll put our most important contributing rule
right at the top: **Always be kind!**

Looking forward to seeing you in the repo! :sparkles:

Flintlock is [MPL-2.0 licensed](LICENSE)

# Table of Contents

<!--
To update the TOC, install https://github.com/kubernetes-sigs/mdtoc
and run: mdtoc -inplace CONTRIBUTING.md
-->

<!-- toc -->
- [Where can I get involved?](#where-can-i-get-involved)
- [Opening Issues](#opening-issues)
  - [Bug report guide](#bug-report-guide)
  - [Feature request guide](#feature-request-guide)
  - [Help request guide](#help-request-guide)
- [Submitting PRs](#submitting-prs)
  - [Choosing something to work on](#choosing-something-to-work-on)
  - [Developing flintlock](#developing-flintlock)
    - [Forking the repo](#forking-the-repo)
    - [Setting up your environment](#setting-up-your-environment)
      - [Go](#go)
    - [Building flintlock](#building-flintlock)
    - [Running the unit tests](#running-the-unit-tests)
    - [Running the end to end tests](#running-the-end-to-end-tests)
      - [In your local environment](#in-your-local-environment)
      - [In a local docker container](#in-a-local-docker-container)
      - [In an Equinix device](#in-an-equinix-device)
    - [Writing your solution](#writing-your-solution)
  - [Asking for help](#asking-for-help)
  - [PR submission guidelines](#pr-submission-guidelines)
    - [Commit message formatting](#commit-message-formatting)
- [How the Maintainers process contributions](#how-the-maintainers-process-contributions)
  - [Prioritizing issues](#prioritizing-issues)
  - [Reviewing PRs](#reviewing-prs)
  - [Dependabot Bundler](#dependabot-bundler)
- [ADRs (Architectural Decision Records)](#adrs-architectural-decision-records)
  - [Process](#process)
- [:rocket: :tada: Thanks for reading! :tada: :rocket:](#rocket-tada-thanks-for-reading-tada-rocket)
<!-- /toc -->

# Where can I get involved?

We are happy to see people in pretty much all areas of flintlock's development.
Here is a non-exhaustive list of ways you can help out:

1. Open a [PR](#submitting-prs). :woman_technologist:

    Beyond fixing bugs and submitting new features, there are other things you can submit
    which, while less flashy, will be deeply appreciated by all who interact with the codebase:

      - Backfilling tests!
      - Refactoring!
      - Reviewing and updating [documentation][user-docs]!

   (See also [Choosing something to work on](#choosing-something-to-work-on) below.)
1. Open an [issue](#opening-issues). :interrobang:

    We have 3 forms of issue: [bug reports](#bug-report-guide), [feature requests](#feature-request-guide) and [help requests](#help-request-guide).
    If you are not sure which category you need, just make the best guess and provide as much information as possible.

1. Help out others in the [community slack channel][slack-channel], or in some [discussions][discussions]. :sos:

1. Chime in on [bugs](https://github.com/weaveworks-liquidmetal/flintlock/issues?q=is%3Aopen+is%3Aissue+label%3Akind%2Fbug+), [feature requests](https://github.com/weaveworks-liquidmetal/flintlock/issues?q=is%3Aopen+is%3Aissue+label%3Akind%2Ffeature) and asks for [help][discussions]. :thought_balloon:

1. Dig into some [`needs-investigation`](https://github.com/weaveworks-liquidmetal/flintlock/labels/needs-investigation) or [`help wanted`](https://github.com/weaveworks-liquidmetal/flintlock/labels/help-wanted) :detective:

1. Get involved in some [PR reviews](https://github.com/weaveworks-liquidmetal/flintlock/pulls). :nerd_face:

# Opening Issues

These guides aim to help you write issues in a way which will ensure that they are processed
as quickly as possible.

_See below for [how issues are prioritized](#prioritizing-issues)_.

**General rules**:

1. Before opening anything, take a good look through existing issues.

1. More is more: give as much information as it is humanly possible to give.
  Highly detailed issues are more likely to be picked up because they can be prioritized and
  scheduled for work faster. They are also more accessible
  to the community, meaning that you may not have to wait for the core team to get to it.

1. Please do not open an issue with a description that is **just** a link to another issue,
  a link to a slack conversation, a quote from either one of those, or anything else
  equally opaque. This raises the bar for entry and makes it hard for the community
  to get involved. Take the time to write a proper description and summarise key points.

1. Take care with formatting. Ensure the [markdown is tidy](https://docs.github.com/en/free-pro-team@latest/github/writing-on-github/getting-started-with-writing-and-formatting-on-github),
  use [code blocks](https://docs.github.com/en/free-pro-team@latest/github/writing-on-github/creating-and-highlighting-code-blocks) etc etc.
  The faster something can be read, the faster it can be dealt with.

1. Keep it civil. Yes, it is annoying when things don't work, but it is way more fun helping out
  someone who is not... the worst. Remember that conversing via text exacerbates
  everyone's negativity bias, so throw in some emoji when in doubt :+1: :smiley: :rocket: :tada:.

**Dedicated guides**:
- [Bug report guide](#bug-report-guide)
- [Feature request guide](#feature-request-guide)
- [Help request guide](#help-request-guide)

## Bug report guide

We hope to get to bug reports within a couple of working days, but please wait for at least
7 before nudging. (Unless it is a super critical end-of-the world bug, then by all means
make some noise :loudspeaker:.)

Below are the criteria we like our bug reports to cover in order to gather the bare minimum of
information. Add more that what is asked for if you can :smiley:.

1. **Search existing issues.** If something similar already exists, and is still open, please contribute to the discussion there.

1. **Bump to the latest version of flintlock** to see whether your issue has already been fixed.

1. **Write a concise and descriptive title**, like you would a commit message, something which others can easily
  find when doing step 1 above.

1. **Detail what it was that you were trying to do and what you expected would happen**.
  Give some background information around what you have already done to your environment, any custom configuration etc.
  With sufficient information you can pre-empt any questions others may have. This should cut out some obvious
  back-and-forth and help get people to the heart of the issue quickly.

1. **Explain what actually happened**. Provide the relevant error message and key logs.

1. **Provide a reproduction**, for example the exact command you started `flintlockd` with,
  or a config file, and anything important in the environment where `flintlockd` is running.
  Please try to reduce your reproduction to the minimal necessary to help whoever is helping you
  get to the broken state without needing to recreate your entire environment.

1. **If possible, reproduce the issue with logging verbosity set to 9** (`--verbosity 9`), if you have not already done so. Ensure
  logs are formatted with [code blocks](https://docs.github.com/en/free-pro-team@latest/github/writing-on-github/creating-and-highlighting-code-blocks).
  If they are long (>50 lines) please provide them in a Gist or collapsed using
  [HTML details tags](https://gist.github.com/ericclemmons/b146fe5da72ca1f706b2ef72a20ac39d).
  Take care to redact any sensitive info.

1. Detail any workarounds which you tried, it may help others who experience the same problem.

1. If you already have a fix in mind, note that on the report and go ahead and open a
  PR whenever you are ready. A core team-member will assign the issue to you.

## Feature request guide

We hope to respond to and prioritize new feature requests within 7 working days. Please wait for
up to 14 before nudging us.

A feature request is the start of a discussion, so don't be put off if it is not
accepted. Features which either do not contribute to or directly work against
the project goals will likely be rejected, as will highly specialised usecases.

Below are the steps we encourage people to take when creating a new feature request:

1. **Search existing issues.** If something similar already exists, and is still open, please contribute to the discussion there.

1. **Explain clearly why you want this feature.**

1. **Describe the behaviour you'd like to see.** As well as an explanation, please
  provide some example commands/config/output. Please ensure everything is formatted
  nicely with [code blocks](https://docs.github.com/en/free-pro-team@latest/github/writing-on-github/creating-and-highlighting-code-blocks).
  If you have strong ideas, be as detailed as you like.

1. Note the deliverable of this issue: should the outcome be a simple PR to implement the
  feature? Or does it need further design or an [ADR](#adrs-architectural-decision-records)?

1. If the feature is small (maybe it is more of an improvement) and you already have
  a solution in mind, explain what you plan to do on the issue and open a PR!
  A core team member will assign the task to you.

## Help request guide

If you are having trouble using flintlockd, but you don't think it is a bug,
please feel free to ask for help with your setup.

While you can ask for general help with `flintlockd` usage in the [slack channel][slack-channel],
starting a [discussion][discussions] creates a more searchable history for the community.

We hope to respond to requests for help within a couple of working days, but please wait
for a week before nudging us.

Please following these steps when starting a new discussion:

1. **Search existing discussion.** If something similar already exists, please contribute to the conversation there.

1. Write a clear title with the format "How to x".

1. Explain what you are trying to accomplish, what you have tried, and the behaviour you are seeing.

1. Please include the exact the commands you're using, and all the steps you took to setup your environment.
   Please ensure everything is formatted nicely with [code blocks](https://docs.github.com/en/free-pro-team@latest/github/writing-on-github/creating-and-highlighting-code-blocks).

1. When providing verbose logs, please use either a Gist or [HTML detail tags](https://gist.github.com/ericclemmons/b146fe5da72ca1f706b2ef72a20ac39d).

It may turn out that it was a bug after all! In which case a new issue will be created.

# Submitting PRs
## Choosing something to work on

If you are not here to report a bug, ask for help or request some new behaviour, this
is the section for you. We have curated a set of issues for anyone who simply
wants to build up their open-source cred :muscle:.

- Issues labelled [`good first issue`](https://github.com/weaveworks-liquidmetal/flintlock/labels/good%20first%20issue)
  should be accessible to folks new to the repo, as well as to open source in general.

  These issues should present a low/non-existent barrier to entry with a thorough description,
  easy-to-follow reproduction (if relevant) and enough context for anyone to pick up.
  The objective should be clear, possibly with a suggested solution or some pseudocode.
  If anything similar has been done, that work should be linked.

  If you have come across an issue tagged with `good first issue` which you think you would
  like to claim but isn't 100% clear, please ask for more info! When people write issues
  there is a _lot_ of assumed knowledge which is very often taken for granted. This is
  something we could all get better at, so don't be shy in asking for what you need
  to do great work :smile:.

  See more on [asking for help](#asking-for-help) below!

- [`help wanted` issues](https://github.com/weaveworks-liquidmetal/flintlock/labels/help%20wanted)
  are for those a little more familiar with the code base, but should still be accessible enough
  to newcomers.

- All other issues labelled `kind/<x>` or `priority/<x>` are also up for grabs, but
  are likely to require a fair amount of context.

## Developing flintlock

**Sections:**
- [Forking the repo](#forking-the-repo)
- [Setting up your environment](#setting-up-your-environment)
- [Building flintlock](#building-flintlock)
- [Running the unit tests](#running-the-unit-tests)
- [Running the end to end tests](#running-the-end-to-end-tests)
- [Writing your solution](#writing-your-solution)

> WARNING: Flintlock is intended and designed to run and be tested on Linux ONLY.
> We provide a `Vagrantfile` for devs who do not have access to a Linux box.

### Forking the repo

Make a fork of this repository and clone it by running:

```bash
git clone git@github.com:<yourusername>/flintlock.git
```

It is not recommended to clone under your `GOPATH` (if you define one), otherwise, you will need to set
`GO111MODULE=on` explicitly.

You may also want to add the original repo to your remotes to keep up to date
with changes.

### Setting up your environment

Follow the [Quick-Start guide][quick-start] to set up your local environment.

Before you begin writing code, you may want to have a play with `flintlock` to get familiar
with the tool, how it works and what it is for. The end of the Quick-Start guide
highlights some tools you can use to interact with the service.

#### Go

This project is written in Go. To be able to contribute you will need:

1. A working Go installation of go 1.18. You can check the
[official installation guide](https://golang.org/doc/install).

2. Make sure that `$(go env GOPATH)/bin` is in your shell's `PATH`. You can do so by
   running `export PATH="$(go env GOPATH)/bin:$PATH"`

3. (Optional, docs contributions only) [User documentation][user-docs] is built
   and generated with [docusaurus](https://docusaurus.io/docs).
   Please make sure you have node installed on your system.

### Building flintlock

Run `make help` to see all development commands.

If you would like to develop flintlock from within a container, use:

```bash
docker run --rm -it \
  --privileged \
  --volume /dev:/dev \
  --volume /run/udev/control:/run/udev/control \
  --volume $(pwd):/src/flintlock \
  --ipc=host \
  --workdir=/src/flintlock \
  weaveworks/flintlock-e2e:latest \
  /bin/bash
```

Note that due to the nature of flintlock, the container will be run with
high privileges and will share some devices and process memory with the host.

### Running the unit tests

To run the tests simply run the following:

```bash
make test
```

The tests at `infrastructure/containerd` require containerd to be running,
but these are disabled unless `CTR_SOCK_PATH` is set on the environment.

To enable these tests:
```bash
# start containerd
export CTR_SOCK_PATH=</path/to/containerd.sock>
make test
```

### Running the end to end tests

See the dedicated docs for the end to end tests [here](test/e2e/README.md).

### Writing your solution

Once you have your environment set up and have completed a clean run of **both** the unit
and the E2E tests you can get to work :tada: .

1. First create a topic branch from where you want to base your work (this is usually
  from `main`):

      ```bash
      git checkout -b <feature-name>
      ```

1. Write your solution. Try to align with existing patterns and standards.

1. Try to commit in small chunks so that changes are well described
  and not lumped in with something unrelated. This will make debugging easier in
  the future.
  Make sure commit messages are in the [proper format](#commit-message-formatting).

1. Ensure that every commit is **compilable** and that the tests run successfully at
  that point.

1. Make sure your commits are [signed](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits).

1. **All code contributions must be submitted with unit tests**. Read [this ADR](https://github.com/weaveworks-liquidmetal/flintlock/discussions/284)
  for our guide on what counts as appropriate testing.
  See [this package](https://github.com/weaveworks-liquidmetal/flintlock/tree/main/core/steps/microvm) package for a good example of tests.

1. For extra special bonus points, if you see any tests missing from the area you are
  working on, please add them! It will be much appreciated :heart: .

1. Check the documentation and update it to cover your changes,
  either in the [README](README.md), [userdocs](userdocs) or [docs](docs/) folder.
  If you have added a completely new feature please ensure that it is documented
  thoroughly.

1. Before you [open your PR](#pr-submission-guidelines), run all the unit and E2E tests and manually
  verify that your solution works.

## Asking for help

If you need help at any stage of your work, please don't hesitate to ask!

- To get more detail on the issue you have chosen, it is a good idea to start by asking
  whoever created it to provide more information.
  If they do not respond, or more help is needed,
  you can then bring in one of the [core maintainers](MAINTAINERS).

- If you are struggling with something while working on your PR, or aren't quite
  sure of your approach, you can open a [draft](https://github.blog/2019-02-14-introducing-draft-pull-requests/)
  (prefix the title with `WIP: `) and explain what you are thinking.
  You can also tag in one of the core team, or drop the PR link into [slack][slack-channel].

- We are happy to pair with contributors over a slack call to help them fine-tune their
  implementation. You can ping us directly, or head to the [channel][slack-channel]
  to see if anyone in the community is up for being a buddy :smiley: .

## PR submission guidelines

Push your changes to the branch on your fork and submit a pull request to the original repository
against the `main` branch.
Where possible, please squash your commits to ensure a tidy and descriptive history.

```bash
git push <remote-name> <feature-name>
```

If your PR is still a work in progress, please open a [Draft PR](https://github.blog/2019-02-14-introducing-draft-pull-requests/)
and prefix your title with the word `WIP`. When your PR is ready for review, you
can change the title and remove the Draft setting.

Our GitHub Actions integration will run the automated tests and give you feedback in the review section. We will review your
changes and give you feedback as soon as possible.

We recommend that you regularly rebase from `main` of the original repo to keep your
branch up to date.

Please ensure that `Allow edits and access to secrets by maintainers` is checked.
While the maintainers will of course wait for you to edit your own work, if you are
unresponsive for over a week, they may add corrections or even complete the work for you,
especially if what you are contributing is very cool :metal: .

PRs which adhere to our guidelines are more likely to be accepted
(when opening the PR, please use the checklist in the template):

1. **The description is thorough.** When writing your description, please be as detailed as possible: don't make people
  guess what you did or simply link back to the issue (the issue explains the problem
  you are trying to solve, not how you solved it.)
  Guide your reviewers through your solution by highlighting
  key changes and implementation choices. Try and pre-empt any obvious questions
  they may have. Providing snippets (or screenshots) of output is very helpful to
  demonstrate new behaviour or UX changes. (Snippets are more searchable than screenshots,
  but we wont be mad at a sneak peek at your terminal envs :eyes: .)

1. **The change has been manually tested.** If you are supplying output above
  then that can be your manual test, with proof :clap: .

1. **The PR has a snappy title**. Your PR title will end up in the release notes,
  so make it a good one. Using the same rule as for the title of a [commit message](#commit-message-formatting)
  is generally a good idea. Try to use the [imperative](https://en.wikipedia.org/wiki/Imperative_mood) and centre it around the behaviour
  or the user value it delivers, rather than any implementation detail.

    eg: `"changed SomeFunc in file.go to also do thing X"`
    is not useful for folks stopping by to quickly see what new stuff they can do with
    `flintlock`, save that for commit messages or the PR description.
    The title `"(feat): Add support for XYZ"` conveys the intent concisely and clearly.

1. **There are new tests for new code.** This is very much required.

1. **There are new tests for old code!** This will earn you the title of "Best Loved
  and Respected Contributor" :boom: :sunglasses: .

1. **There are well-written commit messages** ([see below](#commit-message-formatting))
  which will make future debugging fun. (Any commits of the variety `"fix stuff"`, `"does thing"`
  or `"my god why"` much be squashed or rebased.)

1. **Each of those well-written commits is compilable** and the tests run successfully at
  that point.

In general, we will merge a PR once a maintainer has reviewed and approved it.
Trivial changes (e.g., corrections to spelling) may get waved through.
For substantial changes, more people may become involved, and you might get asked to resubmit the PR or divide the
changes into more than one PR.

### Commit message formatting

_For more on how to write great commit messages, and why you should, check out
[this excellent blog post](https://chris.beams.io/posts/git-commit/)._

We follow a rough convention for commit messages that is designed to answer three
questions: what changed, why was the change made, and how did you make it.

The subject line should feature the _what_ and
the body of the commit should describe the _why_ and _how_.
If you encountered any weirdness along the way, this is a good place
to note what you discovered and how you solved it.

An example of a stellar commit message can be found [here](https://github.com/weaveworks-liquidmetal/flintlock/commit/7a30dd99dc7c05827ba11050505c476799bb2932).

The format can be described more formally as follows:

```text
<short title for what changed>
<BLANK LINE>
<why this change was made and what changed>
<BLANK LINE>
<any interesting details>
<footer>
```

The first line is the subject and should be no longer than 70 characters, the
second line is always blank, and other lines should be wrapped at a max of 80 characters.
This allows the message to be easier to read on GitHub as well as in various git tools.

There is a template recommend for use [here](https://gist.github.com/yitsushi/656e68c7db141743e81b7dcd23362f1a).

# How the Maintainers process contributions

## Prioritizing issues

The core team regularly processes incoming issues. There may be some delay over holiday periods.

Every issue will be assigned a `priority/<x>` label. The levels of priorities are:

- [`critical-urgent`](https://github.com/weaveworks-liquidmetal/flintlock/labels/priority%2Fcritical-urgent): These are time sensitive issues which should be picked up immediately.
  If an issue labelled `critical` is not assigned or being actively worked on,
  someone is expected to drop what they're doing immediately to work on it.
  This usually means the core team, but community members are welcome to claim
  issues at any priority level if they get there first. _However, given the pressing
  timeframe, should a non-core contributor request to be assigned to a `critical` issue,
  they will be paired with a core team-member to manage the tracking, communication and release of any fix
  as well as to assume responsibility of all progess._

- [`important-soon`](https://github.com/weaveworks-liquidmetal/flintlock/labels/priority%2Fimportant-soon): Must be assigned as soon as capacity becomes available.
  Ideally something should be delivered in time for the next release.

- [`important-longterm`](https://github.com/weaveworks-liquidmetal/flintlock/labels/priority%2Fimportant-longterm): Important over the long term, but may not be currently
  staffed and/or may require multiple releases to complete.

- [`backlog`](https://github.com/weaveworks-liquidmetal/flintlock/labels/priority%2Fbacklog): There appears to be general agreement that this would be good to have,
  but we may not have anyone available to work on it right now or in the immediate future.
  PRs are still very welcome, although it might take a while to get them reviewed if
  reviewers are fully occupied with higher priority issues, for example immediately before a release.

These priority categories have been inspired by [the Kubernetes contributing guide](https://github.com/kubernetes/community/blob/master/contributors/guide/issue-triage.md).

Other labels include:

- [`adr-required`](https://github.com/weaveworks-liquidmetal/flintlock/labels/adr-required):
  Indicates that the issue or PR contains a decision that needs to be documented in a [ADR](#adrs-architectural-decision-records) _before_
  it can be worked on.

- [`needs-investigation`](https://github.com/weaveworks-liquidmetal/flintlock/labels/needs-investigation):  There is currently insufficient information to either categorize properly,
  or to understand and implement a solution. This could be because the issue opener did
  not provide enough relevant information, or because more in-depth research is required
  before work can begin.

## Reviewing PRs

The core team aims to clear the PR queue as quickly as possible. Community members
should also feel free to keep an eye on things and provide their own thoughts and expertise.

High-value and/or high priority contributions will be processed as quickly as possible,
while lower priority or nice-to-have things may take a little longer to get approved.

To help facilitate a smoother and faster review, follow the guidelines [above](#pr-submission-guidelines).
Submissions which do not meet standards will be de-prioritised for review.

## Dependabot Bundler

There is an action that will periodically bundle dependabot pull requests into a single
pull request. This PR is not signed and has the label `user-signing-required`. This must be
done by a maintainer of the repository. Check out the PR and push an amending commit into
the existing branch. Then, the PR can be merged.

For an example, take a look at this pull request: [Bundle dependabot pull requests](https://github.com/weaveworks-liquidmetal/flintlock/pull/655).

It was created by [this](https://github.com/weaveworks-liquidmetal/flintlock/actions/runs/4335326798) action triggered on the main branch.

Once the pull request was opened, a maintainer created a commit into it by checking out the branch, then running a `git commit --amend`.

```bash
# git pull if not already up-to-date
git checkout -b bundler-1678007734 origin/bundler-1678007734
git commit --amend
git push -f
```

After this, GitHub will start running the PR actions and it can eventually be merged.

# ADRs (Architectural Decision Records)

Any impactful decisions to the architecture, design, development and behaviour
of flintlock must be recorded in the form of an [ADR](https://engineering.atspotify.com/2020/04/14/when-should-i-write-an-architecture-decision-record/).

A template can be found at [`docs/adr/0000-template.md`][adr-template],
with numerous examples of completed records in the same directory.

Issues for which an ADR is required will be tagged with `adr-required`.
Contributors are also welcome to backfill ADRs if they are found to be missing.

## Process

1. Start a new [discussion][discussions] under the `ADR` category.

1. Choose an appropriate clear and concise title (eg `Implement flintlock in Go`).

1. Open the discussion providing the context of the decision to be made. Describe
  the various options, if more than one, and explain the pros and cons. Highlight
  any areas which you would like the reviewers to pay attention to, or those on which
  you would specifically like an opinion.

1. Tag in the [maintainers](MAINTAINERS) as the "Deciders", and invite them to
  participate and weigh in on the decision and its consequences.

1. Once a decision has been made, open a PR adding a new ADR to the [directory](docs/adr).
  Copy and complete the [template][adr-template];
    - Increment the file number by one
    - Set the status as "Accepted"
    - Set the deciders as those who approved the discussion outcome
    - Summarise the decision and consequences from the discussion thread
    - Link back to the discussion from the ADR doc

# :rocket: :tada: Thanks for reading! :tada: :rocket:

[user-docs]: https://weaveworks-liquidmetal.github.io/flintlock/
[slack-channel]: https://weave-community.slack.com/archives/C02KARWGR7S
[quick-start]: ./docs/quick-start.md
[discussions]: https://github.com/weaveworks-liquidmetal/flintlock/discussions
[adr-template]: ./docs/adr/0000-template.md
