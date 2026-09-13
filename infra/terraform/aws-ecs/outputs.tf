output "ecs_cluster_name" {
  description = "Name of the ECS cluster"
  value       = aws_ecs_cluster.main.name
}

output "ecs_cluster_arn" {
  description = "ARN of the ECS cluster"
  value       = aws_ecs_cluster.main.arn
}

output "alb_dns_name" {
  description = "DNS name of the ECS Application Load Balancer"
  value       = aws_lb.api.dns_name
}

output "alb_arn" {
  description = "ARN of the ECS Application Load Balancer"
  value       = aws_lb.api.arn
}

output "api_service_name" {
  description = "Name of the API ECS service"
  value       = aws_ecs_service.api.name
}

output "worker_service_names" {
  description = "Names of the worker ECS services"
  value       = [for svc in aws_ecs_service.workers : svc.name]
}

output "migration_task_definition_arn" {
  description = "ARN of the DB migration task definition"
  value       = aws_ecs_task_definition.migration.arn
}

output "cloudwatch_log_group_name" {
  description = "CloudWatch log group for ECS tasks"
  value       = aws_cloudwatch_log_group.ecs.name
}
