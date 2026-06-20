import type { Metadata } from "next";
import "./styles.css";

export const metadata: Metadata = {
  title: "Cliks",
  description: "Ambient coworking presence without sharing a single keystroke."
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
