package coralogix

var archiveLogsDataSourceName = "data." + archiveLogsResourceName

//func TestAccCoralogixDataSourceArchiveLogs_basic(t *testing.T) {
//	resource.Test(t, resource.TestCase{
//		PreCheck:                 func() { testAccPreCheck(t) },
//		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceArchiveLogs() +
//					testAccCoralogixDataSourceArchiveLogs_read(),
//				Check: resource.ComposeAggregateTestCheckFunc(
//					resource.TestCheckResourceAttr(archiveLogsDataSourceName, "bucket", "coralogix-c4c-eu2-prometheus-data"),
//					resource.TestCheckResourceAttr(archiveLogsDataSourceName, "active", "true"),
//				),
//			},
//		},
//	})
//}

func testAccCoralogixDataSourceArchiveLogs_read() string {
	return `data "coralogix_archive_logs" "test" {
}
`
}
