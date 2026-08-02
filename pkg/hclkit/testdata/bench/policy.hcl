guardian {
  log_level         = "info"
  dry_run           = false
  worker_count      = 5
  queue_size        = 1000
  schedule_interval = "168h"
  skip_forks        = true
}

locals {
  team    = "platform"
  channel = "#platform-engineering"
  labels  = ["automated", "guardian"]
  footer  = "Need help? Reach out in #platform-engineering."
}

rule "file" "readme" {
  path   = "README.md"
  mode   = "exists"
  team   = local.team
  labels = local.labels
}

rule "file" "codeowners" {
  path   = ".github/CODEOWNERS"
  mode   = "contains"
  team   = local.team
  labels = local.labels

  template = <<EOT
# CODEOWNERS — managed by repo-guardian.
* @${local.team}

${local.footer}
EOT
}

rule "file" "license" {
  path   = "LICENSE"
  mode   = "exists"
  team   = local.team
  labels = local.labels
}

rule "setting" "default_branch" {
  path   = "main"
  mode   = "enforce"
  team   = local.team
  labels = local.labels
}
