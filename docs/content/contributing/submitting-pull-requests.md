## **Contributing to awsctl**

### Ready to Contribute?

Here’s how you can get started:

- **Find Issues to Work On:**

  - Explore our [Priority Issues](https://github.com/berrybytes/awsctl/issues).
  - Check out the [Good First Issues](https://github.com/berrybytes/awsctl/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) for new contributors.
  - Browse the list of [Confirmed Bugs](https://github.com/berrybytes/awsctl/issues?q=is%3Aissue+is%3Aopen+label%3Abug+label%3Aconfirmed) to help us squash those bugs.

- **Set Up Your AWSCTL dev Environment:**
  Follow the [Development Guide](https://github.com/berrybytes/awsctl/blob/main/docs/content/contributing/development-guide.md) to get started.

---

## **Pull Request Submission Process**

### **Pre-Submission Checklist**

Before submitting a PR:

1. **Keep It Simple:**
   - Focus on solving **one problem per PR**.
   - Submit small, manageable pull requests.
2. **Be Thorough:**
   - Add detailed comments explaining your changes.
   - Use semantic line breaks for documentation updates.
3. **Allow Maintainers to Edit:**
   Keep the "Allow edits from maintainers" checkbox checked.

### **Validation Requirements**

- Ensure your PR is **not a draft**.
- Pass:
  - **Validation Checks**
  - **All Tests**
- Receive **2 approving reviews** from maintainers.

---

## **Pull Request Review Cycle**

### **Triage Process**

Each PR undergoes triage to:

- Verify all prerequisites are met.
- Assess whether the use case aligns with project goals.
- Assign reviewers.

### **Review Stages**

1. **Design Review:**
   - Ensures no major conflicts with the codebase.
2. **Code Review:**
   - In-depth review and testing of code.
   - May require changes or clarifications.
   - Contributors are expected to be responsive to feedback.
3. **Merge:**
   - Once approved, the PR is merged automatically or manually.

> ⚠️ During code freezes, PRs may remain unmerged until the release is complete.

---

## **Testing & Merge Workflow**

- Managed by the bot **[Myrmica Lobicornis](https://github.com/traefik/lobicornis)**:
  - Verifies GitHub checks, reviews, and mergeability.
  - Handles branch rebasing/merging if needed.
  - Adds labels like `status/3-needs-merge` to trigger merges.

#### Common Merge Labels:

- `bot/no-merge`: Prevents automatic merging.
- `bot/light-review`: Reduces required LGTM reviews from 2 to 1.

For more details, see the [Lobicornis README](https://github.com/traefik/lobicornis).

---

## **Why Was My PR Closed?**

Your PR may be closed if:

1. **Design Conflicts:**
   The work needed to make it mergeable is too extensive.
   - **Solution:** Create an issue first and involve maintainers during the design phase.
2. **Out-of-Scope Enhancements:**
   The PR is for a feature that doesn't align with project goals.
   - **Solution:** Always create an issue before submitting a PR.
3. **Inactivity:**
   If no feedback is received from the contributor for over **90 days**.

---

## **Why Isn’t My PR Reviewed?**

### Review Prioritization:

1. **Top Priority:**
   - High community engagement and broad applicability.
   - Issues tagged with `contributor/wanted` or `bug/confirmed`.
2. **Secondary Priority:**
   - Smaller enhancements or less critical fixes.
3. **Deprioritized:**
   - Large PRs that are difficult to review.
   - PRs submitted without an associated issue.

> No reviews are conducted during the final weeks of a milestone to reduce churn.

---

## **Best Practices for PRs**

- **Create Issues First:**
  Discuss your proposal with the team before starting work.
- **Submit Small PRs:**
  Break large ideas into smaller, logical PRs.
- **Comment Everything:**
  Assume reviewers are unfamiliar with your perspective or approach.
- **Write Tests:**
  Include adequate tests or ask for help with them.

### Need Help?

- Re-request a review after addressing feedback.
- Politely comment on your PR to bring visibility during triage.

---

## **Handling Feedback**

- **Disagreements:**
  If you believe a requested change is unnecessary, explain your reasoning politely.
- **Overwhelming Comments:**
  Focus on feedback from the primary reviewer (assignee). If needed, ask to fork a new PR to clear outdated comments.

---

## **Resources for Contributors**

- [How to Write a Git Commit Message - Chris Beams](https://chris.beams.io/posts/git-commit/)
- [Git Commit Guidelines](https://git-scm.com/book/en/v2/Distributed-Git-Contributing-to-a-Project)
- [The 50/72 Rule](https://preslav.me/2015/02/21/what-s-with-the-50-72-rule/)
- [A Note About Git Commit Messages - Tim Pope](https://tbaggery.com/2008/04/19/a-note-about-git-commit-messages.html)

---

## **Community Guidelines**

Remember, common sense and courtesy go a long way. Contributing to AWSCTL is a collaborative effort, so let's keep the process respectful and constructive.

Together, we can make AWSCTL the best it can be!

---
