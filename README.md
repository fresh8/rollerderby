## Roller Derby

Roller derby manages rolling replacements of services that use Instance metadata
for the target revision of software.

## Requirements

  * GCP Authentication token.
  * Set appropriate env variables;
    * GOOGLE_APPLICATION_CREDENTIALS="gcp.json" in CI.
    * GOOGLE_PROJECT_ID="your-project"

## Execution

```
rollerderby -key=${METADATA_KEY} -value="${NEW_VALUE}"
```
