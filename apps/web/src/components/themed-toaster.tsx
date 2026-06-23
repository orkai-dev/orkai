import { useEffect, useState } from "react";
import { Toaster } from "sonner";

/** Syncs Sonner with the app's `.dark` class (not just OS preference). */
export function ThemedToaster() {
  const [dark, setDark] = useState(() =>
    typeof document !== "undefined" ? document.documentElement.classList.contains("dark") : true,
  );

  useEffect(() => {
    const root = document.documentElement;
    const observer = new MutationObserver(() => {
      setDark(root.classList.contains("dark"));
    });
    observer.observe(root, { attributes: true, attributeFilter: ["class"] });
    return () => observer.disconnect();
  }, []);

  return <Toaster position="bottom-right" richColors theme={dark ? "dark" : "light"} />;
}
