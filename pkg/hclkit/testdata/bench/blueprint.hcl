name        = "go-api"
description = "Go API service scaffold"
version     = "2.0.0"
tags        = ["go", "api", kebabCase("Service Tag")]

variable "project_name" {
  type        = string
  description = "Name of the project."

  validation {
    condition     = var.project_name != ""
    error_message = "project_name must not be empty"
  }
}

variable "go_module" {
  type    = string
  default = "github.com/example/api"
}

variable "use_grpc" {
  type    = bool
  default = false
}

condition {
  exclude = ["proto/"]
  when    = !var.use_grpc
}

condition {
  exclude = ["docs/grpc.md"]
  when    = !var.use_grpc
}

rename {
  from = "example-module"
  to   = var.go_module
}

rename {
  from = "example-project"
  to   = var.project_name
}
