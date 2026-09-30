import type { ReactNode } from "react";
import en from "../../messages/en.json";

export const metadata = { title: en.app.title };

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
