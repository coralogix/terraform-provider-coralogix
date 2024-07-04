package coralogix

var alertDataSourceName = "data." + alertResourceName

func testAccCoralogixDataSourceAlert_read() string {
	return `data "coralogix_alert" "test" {
	id = coralogix_alert.test.id
}
`
}
