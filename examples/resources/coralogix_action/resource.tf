resource "coralogix_action" action {
  is_private  = false
  source_type = "Log"
  name        = "google search action"
  url         = "https://www.google.com/search?q={{$p.selected_value}}"
}