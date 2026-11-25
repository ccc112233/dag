type JsonSchema = {
  type?: string;
  title?: string;
  description?: string;
  properties?: Record<string, JsonSchema & { enum?: Array<string | number>; default?: unknown; writeOnly?: boolean }>;
  required?: string[];
};

type UiSchema = Record<string, { "ui:widget"?: string }>;

type Props = {
  schema: JsonSchema;
  uiSchema?: UiSchema;
  value: Record<string, unknown>;
  secrets?: Record<string, boolean>;
  onChange: (next: Record<string, unknown>) => void;
  idPrefix?: string;
};

function isPasswordWidget(key: string, prop: JsonSchema & { writeOnly?: boolean }, ui?: UiSchema) {
  if (ui?.[key]?.["ui:widget"] === "password") return true;
  if (prop.writeOnly) return true;
  return /password|token|secret/i.test(key);
}

export function SchemaForm({
  schema,
  uiSchema = {},
  value,
  secrets = {},
  onChange,
  idPrefix = "field",
}: Props) {
  const props = schema.properties || {};
  const required = new Set(schema.required || []);

  return (
    <div className="form-grid">
      {Object.entries(props).map(([key, prop]) => {
        const id = `${idPrefix}-${key}`;
        const label = prop.title || key;
        const password = isPasswordWidget(key, prop, uiSchema);
        const current = value[key];

        if (prop.type === "boolean") {
          return (
            <div className="field" key={key}>
              <label htmlFor={id}>
                <input
                  id={id}
                  type="checkbox"
                  checked={Boolean(current ?? prop.default ?? false)}
                  onChange={(e) => onChange({ ...value, [key]: e.target.checked })}
                />{" "}
                {label}
                {required.has(key) ? " *" : ""}
              </label>
              {prop.description ? <span className="hint">{prop.description}</span> : null}
            </div>
          );
        }

        if (prop.enum) {
          return (
            <div className="field" key={key}>
              <label htmlFor={id}>
                {label}
                {required.has(key) ? " *" : ""}
              </label>
              <select
                id={id}
                value={String(current ?? prop.default ?? "")}
                onChange={(e) => {
                  const raw = e.target.value;
                  const asNum = Number(raw);
                  onChange({
                    ...value,
                    [key]: prop.type === "integer" || prop.enum!.every((x) => typeof x === "number")
                      ? asNum
                      : raw,
                  });
                }}
                required={required.has(key)}
              >
                <option value="">请选择</option>
                {prop.enum.map((opt) => (
                  <option key={String(opt)} value={String(opt)}>
                    {String(opt)}
                  </option>
                ))}
              </select>
            </div>
          );
        }

        const inputType =
          password ? "password" : prop.type === "integer" || prop.type === "number" ? "number" : "text";

        return (
          <div className="field" key={key}>
            <label htmlFor={id}>
              {label}
              {required.has(key) ? " *" : ""}
            </label>
            <input
              id={id}
              type={inputType}
              value={current == null ? "" : String(current)}
              placeholder={
                password && secrets[key]
                  ? "已保存密钥（留空表示不修改）"
                  : prop.default != null
                    ? String(prop.default)
                    : undefined
              }
              onChange={(e) => {
                const raw = e.target.value;
                if (prop.type === "integer") {
                  onChange({ ...value, [key]: raw === "" ? undefined : Number(raw) });
                } else {
                  onChange({ ...value, [key]: raw });
                }
              }}
              required={required.has(key) && !(password && secrets[key])}
              autoComplete={password ? "new-password" : "off"}
            />
            {prop.description ? <span className="hint">{prop.description}</span> : null}
          </div>
        );
      })}
    </div>
  );
}

export function validateRequired(
  schema: JsonSchema,
  value: Record<string, unknown>,
  secrets: Record<string, boolean> = {},
) {
  const missing: string[] = [];
  for (const key of schema.required || []) {
    const prop = schema.properties?.[key];
    const password = prop ? isPasswordWidget(key, prop, {}) : false;
    if (password && secrets[key]) continue;
    const v = value[key];
    if (v === undefined || v === null || v === "") missing.push(key);
  }
  return missing;
}
