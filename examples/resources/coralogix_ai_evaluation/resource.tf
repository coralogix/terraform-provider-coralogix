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

# resource "coralogix_ai_evaluation" "allowed_topics" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     allowed_topics = {
#       topics = [
#         "billing",
#         "account settings"
#       ]
#     }
#   }
# }
#
# resource "coralogix_ai_evaluation" "competition" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     competition = {
#       competitors = [
#         "CompetitorOne",
#         "CompetitorTwo"
#       ]
#     }
#   }
# }
#
# resource "coralogix_ai_evaluation" "restricted_topics" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     restricted_topics = {
#       topics = [
#         "competitor mentions",
#         "medical advice"
#       ]
#     }
#   }
# }
#
# resource "coralogix_ai_evaluation" "sexism" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     sexism = {}
#   }
# }
#
# resource "coralogix_ai_evaluation" "toxicity" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     toxicity = {}
#   }
# }
