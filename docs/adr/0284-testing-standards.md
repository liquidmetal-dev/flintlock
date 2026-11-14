# Testing Practices and Standards

Status: approved
Date: 2021-11-23 (approved on 2022-05-23)
Authors: @Callisto13
Deciders: @Callisto13 @jmickey @richardcase @yitsushi

# Context

The codebase is moving from POC to a production alpha release. Part of getting
production ready is increasing confidence in and quality of the code via automated
tests. As it stands, a fair amount needs to be backfilled, and as the product
grows more will need to be added.
This proposal establishes some base rules and practices which all maintainers
and contributors agree to follow when writing tests.

# Scope

The scope is quite broad, and the topic one which inevitably invites Opinions.
This doc establishes a baseline of what testing elements would constitute an
acceptable PR submission.
This ADR is more capturing what we currently do and giving space to expand,
consider and alter, than me prescribing a new way of doing things.

Topics (include but not limited to):
- When to write tests
- Coverage
- Behaviour
- Mocking
- Refactoring
- Gomega
- Table tests
- Unit tests
- Integration tests
- End to end tests

# Decision

## When to write tests
Always. A PR which adds new or changes existing Go code must always come with
corresponding tests which cover all paths.

## Coverage

All PRs currently run `test-with-cov` which generates a coverage report.

Coverage is a useless metric (unless it is zero) and it is disturbingly easy
to get to 100% without actually testing anything at all.
Furthermore, our report is notoriously inaccurate (coverage appears to change on
pure docs PRs) although this has recently improved.

We have discussed the value of having this report at all, given few of us read
it and it is mostly noise. We did concede that it is useful to have _something_
on PRs purely to check that tests have been added, if for no other reason than
it saves the maintainers/reviewers having to read and nudge.

While again the coverage report should be taken with a bucket of salt, a
variation in the red of more than 5% should raise flags: the submitter should
review their both tests and code to discover gaps.

## True coverage

Generated coverage is not true coverage\*: it indicates that code paths have
been hit... and that's it. Automation will not know, for example, whether
anything is verified after a method call: it will not know whether something
is tested _well_.

\* _Lol yeh true coverage is not a "thing", as really it is subjective and
circumstantial. In this case I mean a shorthand for "coverage of what we
care about", which is; "does my stuff behave the way it should?"_

Tests should be reviewed as critically as any other code, and it is up to the
reviewer to determine whether coverage is sufficient.

Those who T/BDD will naturally get to 100% true coverage with very little
thought or effort.
Those who don't should actively try to break their code and tests after they
have written them.

### Testing behaviour, not lines

In order to get true coverage we test against behaviour rather than
implementation. In other words; we don't test that various internal pathways
have been traversed, we test that the correct set of outcomes have occurred
based on a given set of inputs.

Eg:
- If something is supposed to return an object, validate that the outcome values
  are as expected given the inputs.
- If a method is supposed to take a path and write a file, check that a file
  exists at the location.
- If a function is supposed to throw a catastrophic error if conditions are not
  met, verify the error and message that is returned.

Testing against behaviour also has the benefit of influencing the design of the
code, making it easier to change (and test) in the future. \o/

If we notice that a given line is not covered in any of the test cases and we can see no reason
to add a case for it, we can reasonably ask if that line can be removed. Why is it there?
If it turns out that the line can't be removed, there has to be a reason why it is there,
therefore a behaviour exists which we can test.

### Separate packages

In order to facilitate testing against behaviour, and to ensure that each 'unit'
is actually that (a neatly encapsulated package of code which fulfils a
contract with no sprawling hidden interdependencies), all tests are created in
a separate `_test` package.

### Black-box vs White-box

On a similar theme to the above, we are strongly opposed to black-box testing; none
should be added, in any circumstances. If you are tempted, resist. It always causes pain at the end.

## Mocking

We currently use [gomock](https://github.com/golang/mock) to generate interface mocks,
for our own internal interfaces.

BUT gomock is really crap, so we are making a conscious effort to move
over to [counterfeiter](https://github.com/maxbrunsfeld/counterfeiter).

We shouldn't mock what we don't own. Instead we should create a light shim
around the external component; we integration test against the real thing and
unit test against the mocks of our delegating wrapper.

We should be conscious of overmocking: we should use mocks sparingly when calling
other internal interfaces which are not instrumental to the thing we are testing.
If the interface has a profound effect on the test scenario, we should favour
fakes, stubs and (more sparingly still) the real thing.

## Refactoring

Refactoring tests should be done with a lighter hand than one would refactor
the actual code. While it may make sense to DRY out some tests, if it comes
at a heavy expense of readability or accessibility, then it is not worth it.

## Ginkgo/Gomega

We use the standard Go `testing` package for tests.
We also use [Gomega](https://onsi.github.io/gomega/) matchers to make assertions.

Eg:

```go
Expect(err).NotTo(HaveOccurred()) // note: NotTo is preferred over ToNot
Expect(err).To(HaveOccurred())
Expect(err).To(MatchError("o noes"))

Expect(result.Value).To(Equal(expectedValue))
Expect(outcome).To(BeTrue())

Eventually(something(), "10s").Should(Succeed())
```

See the docs linked above for more examples.

Currently we do not use [Ginkgo](https://onsi.github.io/ginkgo/).

## Table tests

Table tests are simultaneously very useful and the devil's work.

When used incorrectly they can lead to hard-to-read, hard-to-extend, complex
and increasingly brittle tests. A good rule of thumb is: if your test has the
same amount of or more logic than your actual code, then table tests are off
the table (heh heh).

Another one is: is this table being added because it
makes sense to group a bunch of identical tests together, or because the tests
don't feel DRY enough? As noted above, we prioritise readability and stability
over refactoring in tests.

Separate test functions are preferable over complex tables.

## Tests as documentation

A bonus feature of tests is that (when done correctly) they can serve as low
level documentation on how a package/interface/unit behaves.

This makes them a good starting point for new people coming to the codebase.
Human readable tests are also just nicer and easier to maintain.

All test functions must be clearly named, eg:
```go
// bad
func TestMyFunc_1(t *testing.T) {
func TestMyFunc_2(t *testing.T) {

// good
func TestMyFunc_happyPath_returnsFooBar(t *testing.T) {
func TestMyFunc_badInputX_returnsSomeError(t *testing.T) {
```

In table tests, the cases struct should have a `"message"` or `"name"` field,
eg:
```go
// bad
...
    {
    val1:     1478,
    val2:     1834,
    expected: "nooneexpectsthespanishinquisition"
    },
    //  ¯\_(ツ)_/¯
    // what is going on here? do i have to scroll and read the test code like
    // some kind of sucker?
...
// good
...
    {
    name:     "when 'kittens' and 'puppies' expect 'cuteness'",
    val1:     "kitten",
    val2:     "puppy",
    expected: "wowsocute"
    },
...
```

## Test separation

Tests are grouped together, or separated, based on purpose.
For example, each `foo.go` unit of code should be accompanied by
a corresponding `foo_test.go` unit test file.

`foo.go` may not require an integration test or an e2e test, but if any behaviour
in `foo.go` _does_ require those, then they will live in a separate location away from the
unit. Eg. our e2es currently live in `test/e2e/`.

## Unit tests

All code changes must come with a corresponding unit test addition or change.

Units should aim only to test their corresponding unit. While sometimes the
lesser evil is to use real package_Y in package_Z_test, there is nothing
more annoying than making a change in package_X, or package_W, and seeing that
test fail inexplicably.

## Integration tests

Integration tests should be added to test a thin layer of interactions with 3rd
party services. Ours include Firecracker, Containerd and Netlink, as well as any
IO operations.

Not all code changes will require an integration test addition, but changes to
or more of those 3rd party components will.

## End to end tests

E2es mimic the top-level user experience of the product. They are happy-path
CRUD (technically CRD) only and do not check finer variances of behaviour.

Not all code changes will require an E2E addition, but they should be when there
is a change to the end user experience.

# Consequences

* All new code is merged with equivalent new tests covering all new behaviour
* Tests are human readable, accessible and easily extended
* Happy and unhappy behaviours are covered
* Code design co-incidentally improves
* Coverage should always increase (the number is irrelevant, so long as it is
  not zero)
