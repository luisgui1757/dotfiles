# Attach PR evidence

Internal procedure for PR filing and replies. It does not publish anything
without the caller's authority.

1. Confirm the exact artifact and destination. Inspect the image or recording
   for credentials, private application content and unrelated personal data.
2. Identify the hosting service from the remote or PR URL and authenticated
   tool metadata. Account for self-hosted domains. GitHub, GitLab, Azure Repos,
   Bitbucket and CodeCommit have different attachment capabilities.
3. Check current documentation and installed help for that service's upload
   route. Use its supported API, CLI or browser flow. Do not assume a GitHub
   command works elsewhere, upgrade a CLI silently, or create a public bucket,
   gist or throwaway issue to host an attachment.
4. For new PRs or comments, give the validated file to the caller's publication
   operation. Do not call the parent workflow again. Preserve existing body
   content and provide descriptive alt text.
5. Read back the destination and confirm the attachment and body. Inspect its
   rendering when a viewer is available. A local file path is not an uploaded URL.
6. Uploads can partly succeed before returning an error. Inspect remote state
   and retry only the missing artifacts; never duplicate a PR or comment.

If the service has no suitable attachment route, retain the local artifact and
report the limitation. A requested or required attachment remains outstanding.
An optional illustration may be omitted with that limitation disclosed. Do not
claim an artifact was attached unless you observed it at the destination.
