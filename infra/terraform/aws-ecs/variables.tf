variable "aws_region" {
  type        = string
  description = "AWS deployment region"
  default     = "us-east-1"
}

variable "cluster_name" {
  type        = string
  description = "Name of the ECS cluster"
  default     = "vid2vec-ecs"
}

variable "vpc_id" {
  type        = string
  description = "Target VPC ID"
  default     = ""
}

variable "private_subnet_ids" {
  type        = list(string)
  description = "Private subnet IDs for Fargate tasks"
  default     = []
}

variable "public_subnet_ids" {
  type        = list(string)
  description = "Public subnet IDs for the Application Load Balancer"
  default     = []
}

variable "ecr_repository_name" {
  type        = string
  description = "ECR repository name containing the application image"
  default     = "vid2vec"
}

variable "image_tag" {
  type        = string
  description = "Container image tag to deploy"
  default     = "latest"
}

variable "database_url" {
  type        = string
  description = "PostgreSQL connection string"
  default     = ""
  sensitive   = true
}

variable "db_schema" {
  type        = string
  description = "PostgreSQL database schema"
  default     = "discovery_showcase"
}

variable "s3_media_bucket" {
  type        = string
  description = "S3 bucket for media storage"
  default     = ""
}

variable "domain_name" {
  type        = string
  description = "Domain name for ALB routing"
  default     = ""
}

variable "acm_certificate_arn" {
  type        = string
  description = "ACM certificate ARN for HTTPS listener"
  default     = ""
}

variable "api_cpu" {
  type        = number
  description = "CPU units for the API task"
  default     = 256
}

variable "api_memory" {
  type        = number
  description = "Memory in MiB for the API task"
  default     = 512
}

variable "api_desired_count" {
  type        = number
  description = "Desired count for the API service"
  default     = 2
}

variable "worker_cpu" {
  type        = number
  description = "CPU units for each worker task"
  default     = 256
}

variable "worker_memory" {
  type        = number
  description = "Memory in MiB for each worker task"
  default     = 512
}

variable "worker_desired_count" {
  type        = number
  description = "Desired count for each worker service"
  default     = 2
}

variable "tags" {
  type        = map(string)
  description = "Resource tags"
  default = {
    Environment = "production"
    Project     = "vid_2_vec"
    ManagedBy   = "Terraform"
    Platform    = "ECS-Fargate"
  }
}
