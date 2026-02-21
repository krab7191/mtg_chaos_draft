variable "tenancy_ocid" {
  description = "OCI tenancy OCID"
  type        = string
}

variable "user_ocid" {
  description = "OCI user OCID"
  type        = string
}

variable "fingerprint" {
  description = "OCI API key fingerprint"
  type        = string
}

variable "private_key_path" {
  description = "Path to OCI API private key"
  type        = string
}

variable "region" {
  description = "OCI region"
  type        = string
}

variable "compartment_ocid" {
  description = "OCI compartment OCID"
  type        = string
}

variable "availability_domain" {
  description = "OCI availability domain"
  type        = string
}

variable "app_name" {
  description = "Application name"
  type        = string
  default     = "myapp"
}

variable "ssh_public_key" {
  description = "SSH public key for instance access"
  type        = string
}

# Always Free limits: 4 OCPUs, 24GB RAM total across all A1 instances
variable "instance_ocpus" {
  description = "Number of OCPUs"
  type        = number
  default     = 1
}

variable "instance_memory_gb" {
  description = "Memory in GB"
  type        = number
  default     = 2
}
