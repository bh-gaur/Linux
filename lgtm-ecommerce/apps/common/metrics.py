import os
from .traces import is_obs_enabled, service_name

if is_obs_enabled:
    from opentelemetry import metrics
    from opentelemetry.sdk.metrics import MeterProvider
    from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
    from opentelemetry.exporter.otlp.proto.http.metric_exporter import OTLPMetricExporter
    from opentelemetry.sdk.metrics._internal.exemplar import TraceBasedExemplarFilter
    from opentelemetry.sdk.resources import Resource

    otlp_endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4318")
    resource = Resource.create(attributes={"service.name": service_name})

    metric_reader = PeriodicExportingMetricReader(
        OTLPMetricExporter(endpoint=f"{otlp_endpoint}/v1/metrics"),
        export_interval_millis=10000
    )
    meter_provider = MeterProvider(
        metric_readers=[metric_reader],
        resource=resource,
        exemplar_filter=TraceBasedExemplarFilter()
    )
    metrics.set_meter_provider(meter_provider)

    meter = metrics.get_meter(f"{service_name}-meter")
    analysis_counter = meter.create_counter(
        name="python_analysis_ops_total",
        description="Total number of analysis operations executed in Python"
    )
    task_duration_histogram = meter.create_histogram(
        name="python_task_duration_seconds",
        description="Duration of python analysis tasks",
        unit="s"
    )
    processed_items_counter = meter.create_counter(
        name="python_processed_items_total",
        description="Total number of data items processed in Python"
    )
    payment_transactions_total = meter.create_counter(
        name="payment_transactions_total",
        description="Total number of payment transactions processed"
    )
else:
    class NoOpCounter:
        def add(self, amount, attributes=None):
            pass

    class NoOpHistogram:
        def record(self, amount, attributes=None):
            pass

    analysis_counter = NoOpCounter()
    task_duration_histogram = NoOpHistogram()
    processed_items_counter = NoOpCounter()
    payment_transactions_total = NoOpCounter()

