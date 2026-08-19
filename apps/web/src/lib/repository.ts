import { ControlPlaneClient } from "./api-client";
import { flightcheckRepository } from "./demo-data";
import type { FlightcheckRepository } from "./types";

/**
 * Returns the live ControlPlaneClient when CONTROL_PLANE_URL is configured,
 * otherwise the static demo snapshot. This function runs server-side only
 * (called from a Server Component) so the token is never sent to the browser.
 */
export function createRepository(): FlightcheckRepository {
  const baseUrl =
    process.env.CONTROL_PLANE_URL ??
    process.env.NEXT_PUBLIC_CONTROL_PLANE_URL;

  if (!baseUrl) {
    return flightcheckRepository;
  }

  return new ControlPlaneClient({
    baseUrl,
    // Prefer the non-public server-side variable so the token stays out of the
    // client JS bundle; fall back to the public variant for dev convenience.
    apiToken:
      process.env.CONTROL_PLANE_TOKEN ??
      process.env.NEXT_PUBLIC_CONTROL_PLANE_TOKEN ??
      "",
    projectId:
      process.env.CONTROL_PLANE_PROJECT_ID ??
      process.env.NEXT_PUBLIC_CONTROL_PLANE_PROJECT_ID ??
      "",
    runId:
      process.env.CONTROL_PLANE_RUN_ID ??
      process.env.NEXT_PUBLIC_CONTROL_PLANE_RUN_ID ??
      "",
  });
}
