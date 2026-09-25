# Import by user id (UUID). This lists the team to find the user.
terraform import coralogix_user.example 476b96bc-0f2e-42dd-b038-186bc1121b73

# Import by email. This is one lookup, so it is faster for large teams.
terraform import coralogix_user.example someone@example.com
