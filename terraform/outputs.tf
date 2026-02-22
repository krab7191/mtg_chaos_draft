output "server_ip" {
  description = "Public IPv4 address of the server"
  value       = hcloud_server.main.ipv4_address
}

output "ssh_command" {
  description = "SSH command to connect"
  value       = "ssh root@${hcloud_server.main.ipv4_address}"
}
