import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { AcousticProvider } from "../components/AcousticProvider";
import "./styles.css";

const geist = Geist({
  variable: "--font-geist",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Cliks — work alone, together",
  description:
    "A free, open-source CLI that turns your remote team's typing into ambient background sound. No keystrokes shared, no microphones, just presence.",
  icons: { icon: "/images/cliks-keycap.png" },
  openGraph: {
    title: "Cliks — work alone, together",
    description:
      "Free, open-source ambient coworking. Hear your team get things done, without sharing a single keystroke.",
    images: ["/images/warm_desk_workspace.png"],
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${geist.variable} ${geistMono.variable}`}>
      <body className="antialiased min-h-[100dvh] grain">
        <AcousticProvider>{children}</AcousticProvider>
      </body>
    </html>
  );
}
