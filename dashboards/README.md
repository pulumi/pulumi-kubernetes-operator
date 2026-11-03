# Dashboards

Grafana dashboards for the Pulumi Kubernetes Operator.

`operator-metrics.json` shows status on current Stacks, and basic operator
reconcile stats. It is published to the Grafana community catalog as dashboard
[25954](https://grafana.com/grafana/dashboards/25954), owned by the `pulumi`
organization.

To use it, see [the metrics guide](../docs/metrics.md).

## Publishing a revision - Maintainers only

A merge here does not currently update the published copy. 
Upload a revision by hand:

1. Edit the dashboard in this repo, and load it into a local Grafana instance.
   Check to make sure the dashboard looks right.
1. In the "Dashboards" tab in the local Grafana instance, use the "Classic" model and set the export option for sharing externally. 
   This ensures that the dashboard JSON is in the format that grafana.com expects.
1. Increase the `version` field. The catalog orders revisions by this number.
1. Commit the result to this repo so the repo's version matches the version you'll publish in the next step.
1. Log in to grafana.com and publish the Revision under the [dashboard's landing page](https://grafana.com/grafana/dashboards/25954-pulumi-kubernetes-operator/).
