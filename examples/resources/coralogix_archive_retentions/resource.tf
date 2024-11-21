resource "coralogix_archive_retentions" "example" {
  retentions = [
    {
    },
    {
      name = "name_2"
    },
    {
      name = "name_3"
    },
    {
      name = "name_4"
    },
  ]
}
