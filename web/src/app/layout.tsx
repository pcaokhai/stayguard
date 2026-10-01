import type { ReactNode } from "react";
import en from "../../messages/en.json";
import { MockGate } from "mock-layer";

export const metadata = { title: en.app.title };

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <MockGate>{children}</MockGate>
      </body>
    </html>
  );
}
