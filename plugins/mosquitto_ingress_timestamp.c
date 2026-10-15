#define _POSIX_C_SOURCE 200809L

#include <ctype.h>
#include <stdint.h>
#include <string.h>
#include <time.h>

#include <cjson/cJSON.h>
#include <mosquitto.h>

MOSQUITTO_PLUGIN_DECLARE_VERSION(5);

static int timestamp_message(int event, void *event_data, void *userdata)
{
    struct mosquitto_evt_message *message = event_data;
    struct timespec received_at;
    cJSON *event_json;
    const char *parse_end;
    const char *payload_end;
    char *encoded;
    void *payload;
    size_t length;

    (void)event;
    (void)userdata;

    if (strncmp(message->topic, "sensors/", 8) != 0)
        return MOSQ_ERR_SUCCESS;
    if (message->payload == NULL || message->payloadlen == 0)
        return MOSQ_ERR_ACL_DENIED;

    if (clock_gettime(CLOCK_REALTIME, &received_at) != 0)
        return MOSQ_ERR_UNKNOWN;

    payload_end = (const char *)message->payload + message->payloadlen;
    event_json = cJSON_ParseWithLengthOpts(message->payload, message->payloadlen,
                                           &parse_end, 0);
    if (event_json == NULL)
        return MOSQ_ERR_ACL_DENIED;
    while (parse_end < payload_end && isspace((unsigned char)*parse_end))
        parse_end++;
    if (!cJSON_IsObject(event_json) ||
        parse_end != payload_end ||
        cJSON_GetObjectItemCaseSensitive(event_json, "timestamp") != NULL) {
        cJSON_Delete(event_json);
        return MOSQ_ERR_ACL_DENIED;
    }

    double timestamp_ms = (double)received_at.tv_sec * 1000 +
                          received_at.tv_nsec / 1000000;
    if (cJSON_AddNumberToObject(event_json, "timestamp", timestamp_ms) == NULL) {
        cJSON_Delete(event_json);
        return MOSQ_ERR_NOMEM;
    }

    encoded = cJSON_PrintUnformatted(event_json);
    cJSON_Delete(event_json);
    if (encoded == NULL)
        return MOSQ_ERR_NOMEM;

    length = strlen(encoded);
    if (length > UINT32_MAX) {
        cJSON_free(encoded);
        return MOSQ_ERR_INVAL;
    }
    payload = mosquitto_malloc(length);
    if (payload == NULL) {
        cJSON_free(encoded);
        return MOSQ_ERR_NOMEM;
    }
    memcpy(payload, encoded, length);
    cJSON_free(encoded);

    // Mosquitto owns the replacement and frees the original payload.
    message->payload = payload;
    message->payloadlen = (uint32_t)length;
    return MOSQ_ERR_SUCCESS;
}

int mosquitto_plugin_init(mosquitto_plugin_id_t *identifier,
                          void **userdata,
                          struct mosquitto_opt *options,
                          int option_count)
{
    (void)userdata;
    (void)options;
    (void)option_count;

    return mosquitto_callback_register(identifier, MOSQ_EVT_MESSAGE_IN,
                                       timestamp_message, NULL, NULL);
}
