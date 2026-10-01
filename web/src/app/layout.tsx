import type { ReactNode } from "react";
import { Be_Vietnam_Pro } from "next/font/google";
import { MockGate } from "mock-layer";
import { Providers } from "../lib/providers";
import { t } from "../lib/t";
import "../styles/globals.css";

const font = Be_Vietnam_Pro({
  subsets: ["vietnamese", "latin"],
  weight: ["400", "600", "700"],
  variable: "--font-be-vietnam-pro",
  display: "swap",
});

export const metadata = { title: t("app.title") };

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="vi" className={font.variable}>
      <body className="font-sans antialiased">
        <MockGate>
          <Providers>{children}</Providers>
        </MockGate>
      </body>
    </html>
  );
}
