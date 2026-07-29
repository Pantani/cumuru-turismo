resource "aws_vpc" "main" {
  cidr_block           = "10.42.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "${local.name}-vpc"
  }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${local.name}-igw"
  }
}

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.main.id
  availability_zone       = local.availability_zones[0]
  cidr_block              = "10.42.0.0/24"
  map_public_ip_on_launch = false

  tags = {
    Name = "${local.name}-public"
    Tier = "public"
  }
}

resource "aws_subnet" "database" {
  for_each = {
    a = {
      availability_zone = local.availability_zones[0]
      cidr_block        = "10.42.10.0/24"
    }
    b = {
      availability_zone = local.availability_zones[1]
      cidr_block        = "10.42.11.0/24"
    }
  }

  vpc_id                  = aws_vpc.main.id
  availability_zone       = each.value.availability_zone
  cidr_block              = each.value.cidr_block
  map_public_ip_on_launch = false

  tags = {
    Name = "${local.name}-database-${each.key}"
    Tier = "database"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${local.name}-public"
  }
}

resource "aws_route" "public_internet" {
  route_table_id         = aws_route_table.public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.main.id
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

resource "aws_security_group" "application" {
  name        = "${local.name}-application"
  description = "Somente SSH restrito e HTTP/HTTPS publicos"
  vpc_id      = aws_vpc.main.id

  tags = {
    Name = "${local.name}-application"
  }
}

resource "aws_vpc_security_group_ingress_rule" "ssh" {
  security_group_id = aws_security_group.application.id
  description       = "Ansible e operacao"
  cidr_ipv4         = var.admin_cidr
  from_port         = 22
  to_port           = 22
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_ingress_rule" "http" {
  security_group_id = aws_security_group.application.id
  description       = "ACME e redirect HTTPS"
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 80
  to_port           = 80
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_ingress_rule" "https" {
  security_group_id = aws_security_group.application.id
  description       = "Aplicacao HTTPS"
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "application" {
  security_group_id = aws_security_group.application.id
  description       = "Registries, OIDC, OTLP e AWS APIs"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

resource "aws_security_group" "database" {
  name        = "${local.name}-database"
  description = "PostgreSQL somente a partir da aplicacao"
  vpc_id      = aws_vpc.main.id

  tags = {
    Name = "${local.name}-database"
  }
}

resource "aws_vpc_security_group_ingress_rule" "database" {
  security_group_id            = aws_security_group.database.id
  referenced_security_group_id = aws_security_group.application.id
  description                  = "PostgreSQL privado"
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "database" {
  security_group_id = aws_security_group.database.id
  description       = "Respostas de conexoes estabelecidas"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}
