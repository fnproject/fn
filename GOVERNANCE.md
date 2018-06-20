# Fn Project Governance

This document describes the rules and governance of the project. It is meant to be followed by all the developers of the project and the Fn Project community. This document describes how to deal with larger or cross-cutting decisions - technical decisions that are within the remit of one set of maintainers should be discussed in the [Fn Project slack](https://fnrpject.slack.com) or in issues and pull-requests in the appropriate repo.

Common terminology used in this governance document are listed below:
  * Team member: a member of the [Fn Team group on forums.fnproject.io](https://forums.fnproject.io/groups/FnTeam).
  * Maintainer: Maintainers lead an individual project or parts thereof. Specified in MAINTAINERS.md.
  * Project: A single repository in the [Fn Project GitHub organization](https://github.com/fnproject).
  * The Fn Project: The sum of all activities performed under this governance, concerning one or more repositories or the community. 

## Values
Developers and community are expected to follow the values defined in the [Fn Project Code Of Conduct](CODE_OF_CONDUCT.md). Furthermore, the Fn Project community strives for kindness, giving feedback effectively, and building a welcoming environment. The Fn Project developers generally decide by consensus and only resort to conflict resolution by a majority vote if consensus cannot be reached.

## Projects
Each project must have a `MAINTAINERS.md` file with at least one maintainer. Where a project has a release process, access and documentation should be such that more than one person can perform a release. Major releases should be announced in the [Release category in forums.fnproject.io](https://forums.fnproject.io/c/release) . Any new projects should be first proposed in the [Fn Project forums](https://forums.fnproject.io/) following the voting procedures listed below. When a project is no longer relevant it should be moved to the fnproject-junkyard GitHub organization.

## Team members
Team member status may be given to those who have made ongoing contributions to the Fn Project for at least 3 months. This is usually in the form of code improvements and/or notable work on documentation, but organizing events or user support will also be taken into account.

New members may be proposed by any existing member by posting to the Fn Project forums. It is highly desirable to reach consensus about acceptance of a new member. However, the proposal is ultimately voted on by a formal [supermajority vote](#supermajority-vote).

If the new member proposal is accepted, the proposed team member should be contacted privately via email to confirm or decline their acceptance of team membership. This will also be posted to the Fn Project forums for record-keeping purposes.

If they choose to accept, the following steps are taken:
  * Team members are added to the [GitHub organization](https://github.com/fnproject) as _Maintainer_.
  * Team members are added to the [Fn Team group on forums.fnproject.io](https://forums.fnproject.io/groups/FnTeam).
  * New team members are announced on the Fn Project forums by an existing team member.

Team members may retire at any time by posting to the Fn Project forums and removing themselves from the [Fn Team group](https://forums.fnproject.io/groups/FnTeam).

Team members can be removed by [supermajority vote](#supermajority-vote) in the [private Fn Team category in the Fn Project forums](https://forums.fnproject.io/c/fn-team). For this vote, the member in question is not eligible to vote and does not count towards the quorum.

Upon death of a member, their team membership ends automatically.

## Maintainers
Maintainers lead one or more project(s) or parts thereof and serve as a point of conflict resolution amongst the contributors to this project. Ideally, maintainers are also team members, but exceptions are possible for suitable maintainers that, for whatever reason, are not yet team members.

Changes in maintainership have to be announced on the Fn Project forums . They are decided by [lazy consensus](#lazy-consensus) and formalized by changing the `MAINTAINERS.md` file of the respective repository.

Maintainers are granted commit rights to all projects in the [GitHub orgaization](https://github.com/fnproject).

A maintainer may resign by messaging the [Fn Team on the forums](https://forums.fnproject.io/groups/FnTeam) . A maintainer with no project activity for a year is considered to have resigned. Maintainers that wish to resign are encouraged to propose another team member to take over the project.

A project may have multiple maintainers, as long as the responsibilities are clearly agreed upon between them and documented in `MAINTAINERS.md` so that all team members know how it's split. This includes coordinating who handles which issues and pull requests.

## Technical decisions
Technical decisions that only affect a single project are made informally by the maintainer of this project, and [lazy consensus](#lazy-consensus) is assumed. Technical decisions that span multiple parts of the Fn Project should be discussed and made in the [Fn Project forums](https://forums.fnproject.io/).

Decisions are usually made by [lazy consensus](#lazy-consensus). If no consensus can be reached, the matter may be resolved by [majority vote](#majority-vote).

## Governance changes
Material changes to this document are discussed publicly in the [Fn Project forums](https://forums.fnproject.io/). Any change requires a [supermajority](#supermajority-vote) in favor. Editorial changes may be made by [lazy consensus](#lazy-consensus) unless challenged.

## Other matters
Any matter that needs a decision, including but not limited to financial matters, may be called to a vote by any member if they deem it necessary. For financial, private, or personal matters, discussion and voting takes place in the [private Fn Team category in the Fn Project forums](https://forums.fnproject.io/c/fn-team) otherwise in the public [Fn Project forums](https://forums.fnproject.io/).

## Voting
The Fn Project usually runs by informal consensus, however sometimes a formal decision must be made.

Depending on the subject matter, as laid out above, different methods of voting are used.

For all votes, voting must be open for at least seven days. The end date should be clearly stated in the call to vote. A vote may be called and closed early if enough votes have come in one way so that further votes cannot change the final decision.

In all cases, all and only [team members](#team-members) are eligible to vote, with the sole exception of the forced removal of a team member, in which said member is not eligible to vote.

Discussion and votes on personal matters (including but not limited to team membership and maintainership) are held in private in the [private Fn Team category in the Fn Project forums](https://forums.fnproject.io/c/fn-team). All other discussion and votes are held in public in the [Fn Project forums](https://forums.fnproject.io/).

For public discussions, anyone interested is encouraged to participate. Formal power to object or vote is limited to [team members](#team-members).

### Lazy consensus
The default decision making mechanism for the Fn Project is [lazy consensus](https://couchdb.apache.org/bylaws.html#lazy). This means that any decision on technical issues is considered supported by the [team](https://forums.fnproject.io/groups/FnTeam) as long as nobody objects.

Silence on any consensus decision is implicit agreement and equivalent to explicit agreement. Explicit agreement may be stated at will. Decisions may, but do not need to be called out and put up for discussion in the [Fn Project forums](https://forums.fnproject.io/) at any time and by anyone.

Lazy consensus decisions can never override or go against the spirit of an earlier explicit vote.

If any [team member](#team-members) raises objections, the team members work together towards a solution that all involved can accept. This solution is again subject to lazy consensus.

In case no consensus can be found, but a decision one way or the other must be made, any [team member](#team-members) may call a formal [majority vote](#majority-vote).

### Majority vote
Majority votes must be called explicitly as a new topic in the appropriate category in the [Fn Project forums](https://forums.fnproject.io). The topic must be tagged with the 'majority-vote' tag. In the body, the call to vote must state the proposal being voted on. It should reference any discussion leading up to this point.

Votes may take the form of a single proposal, with the option to vote yes or no, or the form of multiple alternatives.

A vote on a single proposal is considered successful if more vote in favor than against.

If there are multiple alternatives, members may vote for one or more alternatives, or vote "no" to object to all alternatives. It is not possible to cast an "abstain" vote. A vote on multiple alternatives is considered decided in favor of one alternative if it has received the most votes in favor, and a vote from more than half of those voting. Should no alternative reach this quorum, another vote on a reduced number of options may be called separately.

### Supermajority vote
Supermajority votes must be called explicitly as a new topic in the appropriate category in the [Fn Project forums](https://forums.fnproject.io). The topic must be tagged with the 'supermajority-vote' tag. In the body, the call to vote must state the proposal being voted on. It should reference any discussion leading up to this point.

Votes may take the form of a single proposal, with the option to vote yes or no, or the form of multiple alternatives.

A vote on a single proposal is considered successful if at least two thirds of those eligible to vote vote in favor.

If there are multiple alternatives, members may vote for one or more alternatives, or vote "no" to object to all alternatives. A vote on multiple alternatives is considered decided in favor of one alternative if it has received the most votes in favor, and a vote from at least two thirds of those eligible to vote. Should no alternative reach this quorum, another vote on a reduced number of options may be called separately.

## FAQ

This section is informational. In case of disagreement, the rules above overrule any FAQ.

### How do I propose a decision?
Propose it in the [Fn Project forums](https://forums.fnproject.io/). If there is no objection within seven days, consider the decision made. If there are objections and no consensus can be found, a vote may be called by a team member.

### How  do I become a team member
To become an official team member, you should make ongoing contributions to one or more project(s) for at least three months. At that point, a team member (typically a maintainer of the project) may propose you for membership. The discussion about this will be held in private, and you will be informed privately when a decision has been made. A possible, but not required, graduation path is to become a maintainer first.

Should the decision be in favor, your new membership will also be announced in the [Fn Project forums](https://forums.fnproject.io/).

### How do I add a project?
As a team member, propose the new project in the [Fn Project forums](https://forums.fnproject.io/). If nobody objects, create the project in the GitHub organization. Add at least a `README.md` explaining the goal of the project, and a `MAINTAINERS.md` with the maintainers of the project (at this point, this probably means you).

### How do I archive or remove a project?

Start a new topic in the [Fn Project forums](https://forums.fnproject.io/) proposing the retirement of a project. If nobody objects within seven days, move it to the fnproject-junkyard GitHub organization.

This governance is based on the [Prometheus project governance](https://prometheus.io/governance/).
