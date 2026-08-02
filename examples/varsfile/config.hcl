variable "environment" {
  type        = string
  description = "Deployment environment."

  validation {
    condition     = var.environment == "dev" || var.environment == "prod"
    error_message = "environment must be dev or prod"
  }
}

variable "replicas" {
  type        = number
  default     = 1
  description = "Instance count."
}

service_name = snakeCase("Demo Service")
environment  = var.environment
replicas     = var.replicas
