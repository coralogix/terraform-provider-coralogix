resource "coralogix_ai_evaluation" "example" {
  application = "my-chatbot"
  subsystem   = "production"
  target      = "response"
  threshold   = 0.8
  is_enabled  = true

  config = {
    pii = {
      categories = [
        "EMAIL_ADDRESS",
        "CREDIT_CARD"
      ]
    }
  }
}
