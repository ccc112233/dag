import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { SchemaForm, validateRequired } from "./SchemaForm";

const schema = {
  type: "object",
  required: ["host", "password"],
  properties: {
    host: { type: "string", title: "主机地址" },
    password: { type: "string", title: "密码", writeOnly: true },
    useSimulator: { type: "boolean", title: "使用模拟器", default: true },
  },
};

describe("SchemaForm", () => {
  it("associates labels with controls", () => {
    render(
      <SchemaForm
        schema={schema}
        value={{ host: "10.0.0.1", useSimulator: true }}
        secrets={{ password: true }}
        onChange={() => undefined}
      />,
    );
    expect(screen.getByLabelText(/主机地址/)).toBeTruthy();
    expect(screen.getByLabelText(/密码/)).toBeTruthy();
    expect(screen.getByLabelText(/使用模拟器/)).toBeTruthy();
  });

  it("skips required secret when already saved", () => {
    expect(validateRequired(schema, { host: "x" }, { password: true })).toEqual([]);
    expect(validateRequired(schema, { host: "" }, {})).toContain("host");
  });
});
