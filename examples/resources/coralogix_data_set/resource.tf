resource "coralogix_data_set" data_set {
  name         = "custom enrichment data"
  description  = "description"
  file_content = file("./date-to-day-of-the-week.csv")
}

resource "coralogix_data_set" data_set2 {
  name        = "custom enrichment data 2"
  description = "description"
  uploaded_file {
    path = "./date-to-day-of-the-week.csv"
  }
}