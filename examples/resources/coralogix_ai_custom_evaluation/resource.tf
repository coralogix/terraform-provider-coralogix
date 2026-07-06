resource "coralogix_ai_custom_evaluation" "example" {
  name                         = "No competitor mentions"
  policy_type                  = "quality"
  description                  = "Flags responses that mention competitor products"
  instructions                 = <<-EOT
  Evaluate whether {response} mentions competitor products.
  Treat each assistant answer independently.
  EOT
  should_include_system_prompt = false
  applications = [{
    application = "my-chatbot"
    subsystem   = "production"
  }]

  criteria = {
    acceptable = {
      flags = <<-EOT
      Does not mention competitor products.
      Answer stays focused on our product.
      EOT
      examples = [
        <<-EOT
        User: which tool should I use?
        Assistant: Our product is a strong fit.
        EOT
      ]
    }

    prohibited = {
      flags = <<-EOT
      Mentions a competitor product.
      Names another vendor as the recommended option.
      EOT
      examples = [
        <<-EOT
        User: which tool should I use?
        Assistant: CompetitorX is a strong fit.
        EOT
      ]
    }
  }
}
