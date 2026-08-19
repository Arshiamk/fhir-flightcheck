import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: {
    default: "FHIR Flightcheck Operations",
    template: "%s · FHIR Flightcheck",
  },
  description:
    "Evidence-backed production readiness checks for FHIR integrations.",
  applicationName: "FHIR Flightcheck",
  keywords: ["FHIR R4", "SMART", "healthtech", "production readiness"],
  robots: { index: false, follow: false },
  openGraph: {
    title: "FHIR Flightcheck Operations",
    description:
      "Blocker-first readiness, reproducible evidence, and policy gates for FHIR integrations.",
    type: "website",
  },
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
