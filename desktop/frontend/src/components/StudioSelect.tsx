import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import { closeStudioSelect, openStudioSelect } from "../lib/studioSelectRegistry";
import { AnchoredPopover } from "./AnchoredPopover";

export type StudioSelectOption = {
  value: string;
  label: string;
  title?: string;
  icon?: ReactNode;
  disabled?: boolean;
};

export function StudioSelect({
  value,
  onChange,
  options,
  label,
  disabled,
  placeholder = "—",
  className = "",
  menuLabel,
  align = "start",
  id: idProp,
  menuClassName = "",
}: {
  value: string;
  onChange: (value: string) => void;
  options: StudioSelectOption[];
  label?: string;
  disabled?: boolean;
  placeholder?: string;
  className?: string;
  menuLabel?: string;
  align?: "start" | "end";
  id?: string;
  menuClassName?: string;
}) {
  const autoId = useId();
  const id = idProp ?? autoId;
  const anchorRef = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const selected = options.find((option) => option.value === value);
  const display = selected?.label ?? placeholder;
  const triggerTitle = selected?.title ?? selected?.label ?? placeholder;

  const closeMenu = useCallback(() => setOpen(false), []);

  useEffect(() => {
    if (!open) return;
    openStudioSelect(closeMenu);
    return () => closeStudioSelect(closeMenu);
  }, [open, closeMenu]);

  return (
    <div className={`studio-select${className ? ` ${className}` : ""}`}>
      {label ? (
        <span className="studio-select__label" id={`${id}-label`}>
          {label}
        </span>
      ) : null}
      <button
        ref={anchorRef}
        type="button"
        id={id}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-labelledby={label ? `${id}-label` : undefined}
        className={`studio-select__trigger${open ? " studio-select__trigger--open" : ""}`}
        title={triggerTitle}
        onClick={() => {
          if (!disabled) setOpen((current) => !current);
        }}
      >
        {selected?.icon ? <span className="studio-select__trigger-icon">{selected.icon}</span> : null}
        <span className="studio-select__trigger-value">{display}</span>
        <ChevronDown size={13} className="studio-select__trigger-caret" aria-hidden />
      </button>
      <AnchoredPopover
        open={open && !disabled}
        anchorRef={anchorRef}
        onClose={closeMenu}
        className="studio-select__popover"
        align={align}
        placement="bottom"
      >
        <div className={`studio-select__menu${menuClassName ? ` ${menuClassName}` : ""}`} role="listbox">
          {menuLabel ? <div className="studio-select__menu-label">{menuLabel}</div> : null}
          {options.map((option) => (
            <button
              key={option.value === "" ? "__empty__" : option.value}
              type="button"
              role="option"
              aria-selected={option.value === value}
              disabled={option.disabled}
              className={`studio-select__item${option.value === value ? " studio-select__item--active" : ""}`}
              title={option.title ?? option.label}
              onClick={() => {
                if (option.disabled) return;
                onChange(option.value);
                closeMenu();
              }}
            >
              {option.icon ? <span className="studio-select__item-icon">{option.icon}</span> : null}
              <span>{option.label}</span>
            </button>
          ))}
        </div>
      </AnchoredPopover>
    </div>
  );
}
