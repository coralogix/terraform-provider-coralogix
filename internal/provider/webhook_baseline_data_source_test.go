package provider

func seedWebhookDataSource(h *grHarness, name string) string {
	return h.api.(*webhookFake).seed(j{
		"name": name,
		"sendLog": j{
			"uuid": webhookUUID, "payload": "test",
		},
	})
}

var webhookDataSourceScenarios = func() map[string]func(*grHarness) {
	cases := nameLookupScenarios(
		seedWebhookDataSource,
		func(h *grHarness, _ string) string { return seedWebhookDataSource(h, "seeded") },
	)
	cases["list-error"] = func(h *grHarness) {
		h.api.(*webhookFake).listStatus = 500
		h.ReadDataSource("list rejected", j{"name": "seeded"})
	}
	cases["get-error"] = func(h *grHarness) {
		id := seedWebhookDataSource(h, "seeded")
		h.api.(*webhookFake).getStatus = 500
		h.ReadDataSource("get rejected", byID(id))
	}
	cases["name-get-error"] = func(h *grHarness) {
		seedWebhookDataSource(h, "seeded")
		h.api.(*webhookFake).getStatus = 500
		h.ReadDataSource("get rejected after name lookup", j{"name": "seeded"})
	}
	cases["route-missing"] = func(h *grHarness) {
		id := seedWebhookDataSource(h, "seeded")
		h.api.(*webhookFake).getStatus = 404
		h.api.(*webhookFake).getMessage = "route not found"
		h.ReadDataSource("get route missing", byID(id))
	}
	cases["blank-id"] = func(h *grHarness) {
		h.ReadDataSource("blank id", j{"id": " "})
	}
	cases["blank-name"] = func(h *grHarness) {
		h.ReadDataSource("blank name", j{"name": " "})
	}
	cases["legacy-teams-by-id"] = func(h *grHarness) {
		id := h.api.(*webhookFake).seed(j{
			"name": "legacy", "microsoftTeams": j{}, "url": "service-value",
		})
		h.ReadDataSource("read legacy Teams", byID(id))
	}
	for _, arm := range []string{
		"custom", "slack", "pager_duty", "sendlog", "email_group",
		"microsoft_teams_workflow", "jira", "opsgenie", "demisto", "event_bridge",
	} {
		cases["type-"+arm] = func(h *grHarness) {
			state := h.Apply("create", h.null(), webhookFullConfigs[arm])
			webhookState(h.t, h, state)
			if arm == "sendlog" {
				h.api.(*webhookFake).setReadURL(idOf(state), "service-value")
			}
			h.ReadDataSource("read by id", byID(idOf(state)))
		}
	}
	return cases
}()
