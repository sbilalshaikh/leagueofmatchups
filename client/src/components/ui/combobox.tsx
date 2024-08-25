"use client";

import * as React from "react";
import { Check, ChevronsUpDown } from "lucide-react";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";

export interface Selectable {
  value: string;
  label: string;
}

interface ComboboxProps {
  selections: Selectable[];
  selectionLabel: string;
  onSelect: (value: string) => void;
  width?: string;
  height?: string;
  labelText?: string;
  labelWidth?: string;
  labelHeight?: string;
}

export function Combobox({
  selections,
  selectionLabel,
  width = "200px",
  height = "40px",
  onSelect,
}: ComboboxProps) {
  const [open, setOpen] = React.useState(false);
  const [value, setValue] = React.useState("");

  return (
    <div className="flex items-center">
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            role="combobox"
            aria-expanded={open}
            className="justify-between"
            style={{ width, height }}
          >
            <span className="truncate">
              {value
                ? selections.find((selection) => selection.value === value)
                    ?.label
                : `Select ${selectionLabel}...`}
            </span>
            <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
          </Button>
        </PopoverTrigger>
        <PopoverContent className="p-0" style={{ width }}>
          <Command>
            <CommandInput
              placeholder={`Search ${selectionLabel.toLowerCase()}...`}
            />
            <CommandList>
              <CommandEmpty>
                No {selectionLabel.toLowerCase()} found.
              </CommandEmpty>
              <CommandGroup>
                {selections.map((selection) => (
                  <CommandItem
                    key={selection.value}
                    value={selection.value}
                    onSelect={(currentValue) => {
                      setValue(currentValue === value ? "" : currentValue);
                      setOpen(false);
                      onSelect(currentValue);
                    }}
                  >
                    <Check
                      className={cn(
                        "mr-2 h-4 w-4",
                        value === selection.value ? "opacity-100" : "opacity-0",
                      )}
                    />
                    {selection.label}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
    </div>
  );
}
