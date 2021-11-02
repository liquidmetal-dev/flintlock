# 4. Merge Create and Update Plan into a Single Plan
<!-- A short and clear title which is prefixed with the ADR number -->

* Status: accepted
* Date: 2021-10-29
* Authors: @jmickey
* Deciders: @jmickey @richardcase @Callisto13 @yitsushi

## Context
<!-- What is the context of the decision and whats the motivation -->

Current each plan is built separately for each type of reconciliation type - e.g. Create, Update, and Delete. 

On 2021-10-26 a meeting was held to consider merging all plans into a single plan - in this case all reconciliation types would generate plans through a single interface. There were some concerns that generating all plans through a single interface would lead to a number of possible issues.

The primary concern is that this would lead to a large amount of logic would exist in a single codepath. With only 3 operations to consider at the moment (create/update/delete) this wouldn't be terrible, but we don't currently have a great understanding of additional operations that might be added moving into the future.

It was identified that a simpler solution would be simply merge the plan builder for the create and update operations, as these operations are closely related, and keep plan building for delete operations on separate codepath, future plans will be decided before implementation.

## Decision
<!-- What is the decision that has been made -->

1. Merge create and update plan generation.
2. Keep plan building for delete operations on separate codepath, future plans will be decided before implementation.

## Consequences
<!-- Whats the result or impact of this decision. Does anything need to change and are new GitHub issues created as a result -->

N/A
