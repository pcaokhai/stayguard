"use client";

import type { ReactNode } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "@/components/ui/drawer";
import { useMediaQuery } from "@/hooks/useMediaQuery";

// Bottom sheet (vaul) on phones, centred dialog from 640 px (docs/16 §3).
export function ResponsiveDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  children: ReactNode;
}) {
  const wide = useMediaQuery("(min-width: 640px)");
  if (wide)
    return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-[480px]">
          <DialogHeader>
            <DialogTitle className="text-[22px]">{title}</DialogTitle>
            <DialogDescription className={description ? undefined : "sr-only"}>
              {description ?? title}
            </DialogDescription>
          </DialogHeader>
          {children}
        </DialogContent>
      </Dialog>
    );
  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <DrawerContent className="max-h-[90dvh] bg-card">
        <DrawerHeader className="text-left">
          <DrawerTitle className="text-[22px]">{title}</DrawerTitle>
          <DrawerDescription className={description ? undefined : "sr-only"}>
            {description ?? title}
          </DrawerDescription>
        </DrawerHeader>
        <div className="flex flex-col gap-3 overflow-y-auto px-5 pb-6">{children}</div>
      </DrawerContent>
    </Drawer>
  );
}
