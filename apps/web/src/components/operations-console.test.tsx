import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { demoSnapshot } from "@/lib/demo-data";
import { OperationsConsole } from "./operations-console";

describe("OperationsConsole", () => {
  it("renders the blocker-first readiness summary with non-color labels", () => {
    render(<OperationsConsole data={demoSnapshot} />);

    expect(
      screen.getByRole("heading", { name: "Release blocked" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Not ready")).toHaveLength(2);
    expect(
      screen.getByRole("link", { name: "Review blockers" }),
    ).toHaveAttribute("href", "#findings");
  });

  it("filters findings and exposes a useful empty state", () => {
    render(<OperationsConsole data={demoSnapshot} />);

    const filters = screen.getByRole("group", { name: "Filter findings" });
    fireEvent.click(within(filters).getByRole("button", { name: "warning" }));
    expect(
      screen.getByRole("button", {
        name: /Duplicate page detected during Patient search/,
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: /Patient write scope exceeds launch context/,
      }),
    ).not.toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("Search findings"), {
      target: { value: "does-not-exist" },
    });
    expect(screen.getByText("No findings match this filter.")).toBeInTheDocument();
  });

  it("updates remediation and linked evidence from keyboard-operable buttons", () => {
    render(<OperationsConsole data={demoSnapshot} />);

    fireEvent.click(
      screen.getByRole("button", {
        name: /AI workflow attempted an unapproved clinical write/,
      }),
    );

    expect(
      screen.getByRole("heading", {
        name: "AI workflow attempted an unapproved clinical write",
      }),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: /Guardrailed tool trace/ }),
    );
    expect(
      screen.getByRole("heading", { name: "Guardrailed tool trace" }),
    ).toBeInTheDocument();
  });
});
