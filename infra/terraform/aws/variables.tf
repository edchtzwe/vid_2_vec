variable "aws_region" {
  type        = string
  description = "AWS deployment region"
  default     = "us-east-1"
}

variable "vpc_cidr" {
  type        = string
  description = "CIDR block for the VPC"
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  type        = list(string)
  description = "List of availability zones for multi-AZ deployment"
  default     = ["us-east-1a", "us-east-1b"]
}

variable "cluster_name" {
  type        = string
  description = "Name of the EKS cluster"
  default     = "vid2vec-eks-cluster"
}

variable "node_instance_types" {
  type        = list(string)
  description = "Instance types for EKS managed node group"
  default     = ["t3.medium"]
}

variable "min_node_count" {
  type        = number
  description = "Minimum node count for failover across AZs"
  default     = 2
}

variable "max_node_count" {
  type        = number
  description = "Maximum node count"
  default     = 5
}

variable "desired_node_count" {
  type        = number
  description = "Desired node count"
  default     = 2
}

variable "crossplane_namespace" {
  type        = string
  description = "Kubernetes namespace for Crossplane"
  default     = "crossplane-system"
}

variable "ecr_repository_name" {
  type        = string
  description = "ECR repository name for the application container image"
  default     = "vid2vec"
}

variable "crossplane_chart_version" {
  type        = string
  description = "Helm chart version for Crossplane"
  default     = "1.16.0"
}

variable "tags" {
  type        = map(string)
  description = "Resource tags"
  default = {
    Environment = "production"
    Project     = "vid_2_vec"
    ManagedBy   = "Terraform"
  }
}

variable "domain_name" {
  type        = string
  description = "Primary domain name for Route53 and ACM certificate"
  default     = "vid2vec.example.com"
}
