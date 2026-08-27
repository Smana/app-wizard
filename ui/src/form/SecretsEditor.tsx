// Secret guardrail editor (FR-010 / US-6). There is deliberately NO "secret
// value" input. Only two things can be added:
//   (a) non-sensitive literal env vars (name + value)
//   (b) ExternalSecret references (name + a path in the platform's secret store)
// Secret *values* live in the secret store and are pulled at runtime by the
// External Secrets Operator — never committed to Git.
//
// Which secret store that is (AWS Secrets Manager, GCP Secret Manager, Vault, …)
// is the platform's business, not the wizard's: the copy below reads it off the
// XRD's own field descriptions. Naming one provider here would be wrong on every
// deployment that uses a different one — and this repo now backs both an AWS and
// a GCP composition.
import type { PathSeg } from "./model";
import { deleteAt, getAt, setAt } from "./model";
import { asSchema, type JSONSchema } from "./jsonSchema";
import { Alert, AlertDescription, AlertTitle } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { RemoveButton } from "./Field";

interface EnvVar {
  name?: string;
  value?: string;
}
interface SecretRef {
  name?: string;
  remoteRef?: string;
}

interface Props {
  spec: unknown;
  onChange: (next: unknown) => void;
  envPath: PathSeg[];
  secretsPath: PathSeg[];
  // The XRD's schema for the external-secrets field. Supplies the platform's own
  // wording for what the reference points at, so this widget names no provider.
  secretsSchema?: JSONSchema;
}

export function SecretsEditor({ spec, onChange, envPath, secretsPath, secretsSchema }: Props) {
  const env = (getAt(spec, envPath) as EnvVar[] | undefined) ?? [];
  const secrets = (getAt(spec, secretsPath) as SecretRef[] | undefined) ?? [];

  const itemSchema = secretsSchema ? asSchema(asSchema(secretsSchema).items) : undefined;
  const remoteRefSchema = itemSchema?.properties?.remoteRef
    ? asSchema(itemSchema.properties.remoteRef)
    : undefined;
  const remoteRefHelp = remoteRefSchema?.description;
  const remoteRefPlaceholder = remoteRefHelp ?? "path in the secret store";

  return (
    <div className="space-y-4">
      <Alert variant="info">
        <AlertTitle>Secrets never transit the wizard</AlertTitle>
        <AlertDescription className="text-pretty">
          Put <strong>non-sensitive</strong> configuration in literal env vars. For anything secret
          (tokens, passwords, keys), add an <strong>ExternalSecret reference</strong> pointing at a
          path in your platform's secret store — the value stays there and is injected at runtime by
          the External Secrets Operator. This form has no field for a secret value.
        </AlertDescription>
      </Alert>

      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">Environment variables</span>
          <span className="text-xs text-muted-foreground">non-sensitive literals only</span>
        </div>
        {env.map((row, i) => (
          <div key={i} className="flex gap-2">
            <Input
              aria-label="Variable name"
              placeholder="NAME"
              value={row.name ?? ""}
              onChange={(e) => onChange(setAt(spec, [...envPath, i, "name"], e.target.value || undefined))}
            />
            <Input
              aria-label="Variable value"
              placeholder="value"
              value={row.value ?? ""}
              onChange={(e) =>
                onChange(setAt(spec, [...envPath, i, "value"], e.target.value || undefined))
              }
            />
            <RemoveButton
              label={`Remove ${row.name || `env var ${i + 1}`}`}
              onClick={() => onChange(deleteAt(spec, [...envPath, i]))}
            />
          </div>
        ))}
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => onChange(setAt(spec, [...envPath, env.length], {}))}
        >
          + Add env var
        </Button>
      </div>

      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">Secret references</span>
        </div>
        {remoteRefHelp && (
          <p className="text-xs text-pretty text-muted-foreground">{remoteRefHelp}</p>
        )}
        {secrets.map((row, i) => (
          <div key={i} className="flex gap-2">
            <Input
              aria-label="Secret name"
              placeholder="ENV / secret name"
              value={row.name ?? ""}
              onChange={(e) =>
                onChange(setAt(spec, [...secretsPath, i, "name"], e.target.value || undefined))
              }
            />
            <Input
              aria-label="Secret store path"
              placeholder={remoteRefPlaceholder}
              value={row.remoteRef ?? ""}
              onChange={(e) =>
                onChange(setAt(spec, [...secretsPath, i, "remoteRef"], e.target.value || undefined))
              }
            />
            <RemoveButton
              label={`Remove ${row.name || `secret reference ${i + 1}`}`}
              onClick={() => onChange(deleteAt(spec, [...secretsPath, i]))}
            />
          </div>
        ))}
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => onChange(setAt(spec, [...secretsPath, secrets.length], {}))}
        >
          + Add secret reference
        </Button>
      </div>
    </div>
  );
}
