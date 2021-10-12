# Contributing

We highly value and encourage contributions from the community!

Reignite is [Apache 2.0 licensed](LICENCE) and accepts contributions via GitHub
Pull Requests.This document outlines some of the conventions on development
workflow, commit message formatting, contact points and other resources to make
it easier to get your contribution accepted.

We gratefully welcome improvements to documentation as well as to code.

## Guidelines

If you have a feature suggestion or found a bug, head over to
[GitHub issues][issues] and see if there's an open issue matching your
description. If not feel free to open a new issue and add short description:
 - In case of a bug, be sure to include the steps you performed and what Reignite responded so it's easy for others to reproduce
 - If you have a feature suggestion, describe it in moderate detail and include some potential uses you see for the feature.
   We prioritize the features to be implemented based on their
   usefulness/popularity. Of course if you want to start contributing yourself,
   go ahead! We'll be more than happy to review your pull requests.

The maintainers will add the correct labels/milestones to the issue for you.

[issues]: https://github.com/weaveworks/reignite/issues

### Contributing your code

The process to contribute code to Reignite is very straightforward.

1. Go to the project on [GitHub][repo] and click the `Fork` button in the
   top-right corner. This will create your own copy of the repository in your
   personal account.
2. Using standard `git` workflow, `clone` your fork, make your changes and then
   `commit` and `push` them to your repository.
3. Run `make generate`, then `commit` and `push` the changes.
4. Go back to [GitHub][repo], select `Pull requests` from the top bar and click
   `New pull request` to the right. Select the `compare across forks` link.
   This will show repositories in addition to branches.
5. From the `head repository` dropdown, select your forked repository. If you
   made a new branch, select it in the `compare` dropdown. You should always
   target `weaveworks/reignite` and `main` as the base repository and branch.
6. With your changes visible, click `Create pull request`. Give it a short,
   descriptive title and write a comment describing your changes. Click `Create
   pull request`.

That's it! Maintainers follow pull requests closely and will add the correct
labels and milestones. After a maintainer's review small changes/improvements
could be requested, don't worry, feedback can be easily addressed by performing
the requested changes and doing a commit and push. Your new changes will
automatically be added to the pull request.

We also have Continuous Integration (CI) set up (powered by [GitHub
Actions][gha]) that will build the code and verify it compiles and passes all
tests successfully. If your changes didn't pass CI, you can click Details to go
and check why it happened. To integrate your changes, we require CI to pass.

[repo]: https://github.com/weaveworks/reignite
[gha]: https://github.com/weaveworks/reignite/actions
