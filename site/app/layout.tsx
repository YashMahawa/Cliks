import type { Metadata } from "next";
import { Syne, JetBrains_Mono } from "next/font/google";
import { AcousticProvider } from "../components/AcousticProvider";
import "./styles.css";

const display = Syne({
  variable: "--font-display",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700", "800"],
});

const mono = JetBrains_Mono({
  variable: "--font-mono-face",
  subsets: ["latin"],
  weight: ["400", "500"],
});

export const metadata: Metadata = {
  metadataBase: new URL(process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000"),
  title: "Cliks - work alone, together",
  description:
    "A free, open-source CLI that turns your remote team's typing into ambient background sound. No keystrokes shared, no microphones, just presence.",
  icons: { icon: "/images/cliks-keycap.png" },
  openGraph: {
    title: "Cliks - work alone, together",
    description:
      "Free, open-source ambient coworking. Hear your team get things done, without sharing a single keystroke.",
    images: ["/images/warm_desk_workspace.png"],
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    // suppressHydrationWarning: browser extensions (Dark Reader, etc.) inject
    // attributes on <html>/<body> before React hydrates. That is not app state.
    <html lang="en" className={`${display.variable} ${mono.variable}`} suppressHydrationWarning>
      <body className="min-h-[100dvh] bg-page text-fg antialiased" suppressHydrationWarning>
        <AcousticProvider>{children}</AcousticProvider>
      </body>
    </html>
  );
}
