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
  title: "Cliks",
  description: "Ambient coworking presence without sharing a single keystroke."
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${geist.variable} ${geistMono.variable}`}>
      <body className="antialiased min-h-[100dvh]">
        <AcousticProvider>{children}</AcousticProvider>
      </body>
    </html>
  );
}
