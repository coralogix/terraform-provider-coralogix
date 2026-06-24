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
# resource "coralogix_ai_evaluation" "language_mismatch" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     language_mismatch = {}
#   }
# }
#
# resource "coralogix_ai_evaluation" "hallucination_completeness" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     hallucination_completeness = {}
#   }
# }
#
# resource "coralogix_ai_evaluation" "hallucination_context_adherence" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     hallucination_context_adherence = {}
#   }
# }
#
# resource "coralogix_ai_evaluation" "hallucination_context_relevance" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     hallucination_context_relevance = {}
#   }
# }
#
# resource "coralogix_ai_evaluation" "hallucination_correctness" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     hallucination_correctness = {}
#   }
# }
#
# resource "coralogix_ai_evaluation" "hallucination_task_adherence" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     hallucination_task_adherence = {}
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
# resource "coralogix_ai_evaluation" "prompt_injection" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "prompt"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     prompt_injection = {
#       additional_context = "Only inspect the user prompt."
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
# resource "coralogix_ai_evaluation" "sql_allowed_tables" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     sql_allowed_tables = {
#       tables = [
#         "orders",
#         "customers"
#       ]
#     }
#   }
# }
#
# resource "coralogix_ai_evaluation" "sql_hallucination" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     sql_hallucination = {}
#   }
# }
#
# resource "coralogix_ai_evaluation" "sql_read_only" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     sql_read_only = {}
#   }
# }
#
# resource "coralogix_ai_evaluation" "sql_restricted_tables" {
#   application = "my-chatbot"
#   subsystem   = "production"
#   target      = "response"
#   threshold   = 0.8
#   is_enabled  = true
#
#   config = {
#     sql_restricted_tables = {
#       tables = [
#         "secrets",
#         "audit_logs"
#       ]
#     }
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
