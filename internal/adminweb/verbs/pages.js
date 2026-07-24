(function () {
  "use strict";

  const widget = require("widget.dsl");
  const admin = require("tinyidp.admin");
  const data = admin.pageData();
  const title = String(data.title || "TinyIDP Console");
  const metrics = Array.isArray(data.metrics) ? data.metrics : [];
  const rows = Array.isArray(data.rows)
    ? data.rows.map((row) => ({
        id: String(row.id || ""),
        primary: String(row.primary || ""),
        secondary: String(row.secondary || ""),
        status: String(row.status || ""),
      }))
    : [];

  const page = widget.page({ id: String(data.id || "overview"), title }, (builder) => {
    builder.section("Overview", (section) => {
      metrics.forEach((metric) => section.metric(String(metric.label), String(metric.value)));
      if (data.message) {
        section.view(
          widget.ui.callout(
            { tone: String(data.tone || "info"), title: String(data.messageTitle || "Status") },
            String(data.message),
          ),
        );
      }
      return section;
    });

    if (rows.length) {
      const schema = widget.data
        .fields("records", (fields) =>
          fields
            .key("id")
            .primary("primary", { label: String(data.primaryLabel || "Name") })
            .short("secondary", { label: String(data.secondaryLabel || "Details") })
            .status("status", { label: "Status" }),
        )
        .build();
      const collection = widget.data
        .collection("records", rows, (collectionBuilder) =>
          collectionBuilder.schema(schema).table((table) => table),
        )
        .toNode();
      builder.section(String(data.tableTitle || "Records"), (section) => section.view(collection));
    }
  });
  return page.toPage();
})()
