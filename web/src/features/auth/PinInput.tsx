"use client";

import { REGEXP_ONLY_DIGITS } from "input-otp";
import { Shake } from "@/components/motion";
import { InputOTP, InputOTPGroup, InputOTPSlot } from "@/components/ui/input-otp";
import { PIN_LENGTH } from "./pin";

// Six masked slots. `shake` counts wrong attempts: each change shakes the group (docs/16 §5).
export function PinInput({
  value,
  onChange,
  shake = 0,
  invalid,
  id,
  onComplete,
}: {
  value: string;
  onChange: (v: string) => void;
  shake?: number;
  invalid?: boolean;
  id?: string;
  onComplete?: () => void;
}) {
  return (
    <Shake trigger={shake}>
      <InputOTP
        id={id}
        maxLength={PIN_LENGTH}
        pattern={REGEXP_ONLY_DIGITS}
        inputMode="numeric"
        autoComplete="current-password"
        value={value}
        onChange={onChange}
        onComplete={onComplete}
        containerClassName="w-full"
      >
        <InputOTPGroup className="w-full gap-2">
          {Array.from({ length: PIN_LENGTH }, (_, i) => (
            <InputOTPSlot
              key={i}
              index={i}
              mask
              aria-invalid={invalid}
              className="h-[52px] flex-1 rounded-[10px] border bg-card text-xl first:rounded-l-[10px] last:rounded-r-[10px] first:border-l"
            />
          ))}
        </InputOTPGroup>
      </InputOTP>
    </Shake>
  );
}
