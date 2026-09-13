output "cluster_name" {
  description = "EKS Cluster Name"
  value       = aws_eks_cluster.main.name
}

output "cluster_endpoint" {
  description = "EKS API Endpoint"
  value       = aws_eks_cluster.main.endpoint
}

output "cluster_security_group_id" {
  description = "Security group ID attached to the EKS cluster"
  value       = aws_eks_cluster.main.vpc_config[0].cluster_security_group_id
}

output "oidc_provider_arn" {
  description = "OIDC Provider ARN for IAM Roles for Service Accounts (IRSA)"
  value       = aws_iam_openid_connect_provider.eks.arn
}

output "crossplane_role_arn" {
  description = "IAM Role ARN for Crossplane Provider-AWS IRSA"
  value       = aws_iam_role.crossplane_provider_aws.arn
}

output "ecr_repository_url" {
  description = "ECR repository URL for the application container image"
  value       = aws_ecr_repository.app.repository_url
}

output "private_subnet_ids" {
  description = "Private Subnet IDs"
  value       = aws_subnet.private[*].id
}
