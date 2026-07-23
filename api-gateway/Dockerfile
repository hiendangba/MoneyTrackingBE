FROM envoyproxy/envoy:v1.39.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends gettext-base \
    && rm -rf /var/lib/apt/lists/*

COPY envoy.yaml /etc/envoy/envoy.yaml
COPY envoy.render.yaml.template /etc/envoy/envoy.render.yaml.template
COPY docker-entrypoint.sh /usr/local/bin/money-tracking-envoy-entrypoint

RUN chmod 0555 /usr/local/bin/money-tracking-envoy-entrypoint

EXPOSE 80 443 9901

ENTRYPOINT ["/usr/local/bin/money-tracking-envoy-entrypoint"]
