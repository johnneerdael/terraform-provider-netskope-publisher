resource "npa_publisher" "primary" {
  name = "pub-eu-1"
}

resource "npa_publisher_token" "primary" {
  publisher_id = npa_publisher.primary.id
}

output "token" {
  value     = npa_publisher_token.primary.token
  sensitive = true
}
