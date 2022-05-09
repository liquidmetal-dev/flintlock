# 3. MicroVM Spec Validation

* Status: Proposed
* Date: 2021-10-14
* Authors: @jmickey
* Deciders: @jmickey @richardcase

## Context

MicroVM specs are provided via the gRPC API. When a spec is received gRPC does basic type validation, but data validation is still required to ensure that the provided spec doesn't contain unsupport parameters.

There are some Protobuf level validation plugins which would allow us to set validation rules within the Protobuf specification itself, and the resulting validation Go code would be generated for us.

The two most relevant plugins found are:

- https://github.com/envoyproxy/protoc-gen-validate
- https://github.com/mwitkow/go-proto-validators

Unfortunately, both of these plugins do not currently support the `optional` proto3 keyword. Additionally, [envoyproxy/protoc-gen-validate](https://github.com/envoyproxy/protoc-gen-validate) is still in alpha, and therefore the API is not stable. [mwitkow/go-proto-validators](https://github.com/mwitkow/go-proto-validators) on the other hand has not been updated in some time, with the last commit in Aug 2020, so it appears to be abandonware.

As a result, neither of these solutions is fit for purpose.

Alternative options to utilising these Protobuf plugins are:

1. Fork one of the above plugins and maintain it for our own purposes.
2. Perform validation on the model. This would be done either in the implementation of the use case - e.g. https://github.com/weaveworks-liquidmetal/flintlock/blob/main/core/application/commands.go#L14, or the conversion from the request type to the model: e.g. https://github.com/weaveworks-liquidmetal/flintlock/blob/main/infrastructure/grpc/server.go#L33.
  - For this we could utilise the [go-playground/validator](https://github.com/go-playground/validator) project.

## Decision

Forking the Protobuf plugins is not currently feasible given both the small size of the team and current project prioritisation. It is also quite a "heavy-handed" solution to our problem.

**Performing validation on the model** is a simpler solution that both solves our problem and remains consistent with our goals in terms of speed of development.

We should investigate and utilise the [go-playground/validator](https://github.com/go-playground/validator) project if we find it is suitable for our use case.

## Consequences

We should continue to revisit this decision periodically. There would still be value in being able to do early request level validation via a gRPC interceptor. We should monitor the Protobuf validation plugins mentioned above and reevaluate their fit for purpose if/when they are updated.