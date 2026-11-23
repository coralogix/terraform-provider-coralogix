# Import by user id (UUID). This lists the team to find the user.
terraform import coralogix_user.example 476b96bc-0f2e-42dd-b038-186bc1121b73

# Import by email. The import resolves the name through the Search username filter,
# then reads the user by its id.
terraform import coralogix_user.example someone@example.com
