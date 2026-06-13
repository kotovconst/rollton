data "aws_caller_identity" "current" {}

module "network" {
  source = "../../modules/network"

  name_prefix = var.name_prefix
  azs         = local.azs
  tags        = local.common_tags
}

module "bot_host" {
  source = "../../modules/bot_host"

  name_prefix      = var.name_prefix
  aws_region       = var.aws_region
  vpc_id           = module.network.vpc_id
  subnet_id        = module.network.public_subnet_ids[0]
  instance_type    = var.bot_instance_type
  ssh_public_key   = var.ssh_public_key
  operator_ip_cidr = var.operator_ip_cidr
  ssm_param_prefix = local.ssm_prefix
  api_domain       = local.api_domain
  ghcr_owner       = var.ghcr_owner
  bots             = local.bots
  tags             = local.common_tags
}

module "database" {
  source = "../../modules/database"

  name_prefix           = var.name_prefix
  vpc_id                = module.network.vpc_id
  subnet_ids            = module.network.public_subnet_ids
  bot_security_group_id = module.bot_host.security_group_id
  instance_class        = var.db_instance_class
  allocated_storage     = var.db_allocated_storage
  tags                  = local.common_tags
}

module "secrets" {
  source = "../../modules/secrets"

  prefix       = local.ssm_prefix
  database_url = module.database.connection_string
  bot_names    = [for b in local.bots : b.name]
  tags         = local.common_tags
}

module "dns" {
  source = "../../modules/dns"

  cloudflare_zone_id = var.cloudflare_zone_id
  api_target_ip      = module.bot_host.public_ip
  app_target_cname   = var.cloudflare_pages_cname
}

module "github_oidc" {
  source = "../../modules/github_oidc"

  github_repo = var.github_repo
  tags        = local.common_tags
}
