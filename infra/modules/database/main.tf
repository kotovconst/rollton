resource "random_password" "master" {
  length  = 32
  special = false # avoid characters that need URL-encoding
}

resource "aws_db_subnet_group" "this" {
  name       = "${var.name_prefix}-db"
  subnet_ids = var.subnet_ids
  tags       = var.tags
}

resource "aws_security_group" "db" {
  name        = "${var.name_prefix}-db"
  description = "Postgres access — bot host only"
  vpc_id      = var.vpc_id
  tags        = var.tags
}

resource "aws_vpc_security_group_ingress_rule" "db_from_bot" {
  security_group_id            = aws_security_group.db.id
  referenced_security_group_id = var.bot_security_group_id
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
  description                  = "Postgres from bot host"
}

resource "aws_db_instance" "this" {
  identifier             = "${var.name_prefix}-pg"
  engine                 = "postgres"
  engine_version         = var.engine_version
  instance_class         = var.instance_class
  allocated_storage      = var.allocated_storage
  storage_type           = "gp3"
  storage_encrypted      = true
  db_name                = var.db_name
  username               = var.master_username
  password               = random_password.master.result
  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.db.id]
  publicly_accessible    = false
  multi_az               = false

  backup_retention_period   = 7
  backup_window             = "02:00-03:00"
  maintenance_window        = "Sun:03:30-Sun:04:30"
  copy_tags_to_snapshot     = true
  skip_final_snapshot       = true # pre-revenue convenience; flip to false later
  final_snapshot_identifier = null

  apply_immediately          = false
  auto_minor_version_upgrade = true

  tags = merge(var.tags, { Name = "${var.name_prefix}-pg" })
}
