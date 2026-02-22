variable "hcloud_token" {
  description = "Hetzner Cloud API token"
  type        = string
  sensitive   = true
}

variable "app_name" {
  description = "Application name (used for resource naming)"
  type        = string
  default     = "mtg-chaos-draft"
}

variable "server_type" {
  description = "Hetzner server type"
  type        = string
  default     = "cax11"
}

variable "location" {
  description = "Hetzner datacenter location"
  type        = string
  default     = "nbg1"
}

variable "ssh_key_name" {
  description = "Name of an existing SSH key in your Hetzner project (Cloud Console → Security → SSH Keys)"
  type        = string
}
