output "instance_public_ip" {
  description = "Public IP of the instance"
  value       = oci_core_instance.main.public_ip
}

output "ssh_command" {
  description = "SSH command to connect"
  value       = "ssh ubuntu@${oci_core_instance.main.public_ip}"
}
