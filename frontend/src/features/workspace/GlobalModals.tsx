import React, { useState, useEffect } from 'react';
import { ProjectPickerModal } from './ProjectPickerModal';
import { OpenExistingProjectModal } from './OpenExistingProjectModal';

export const GlobalModals: React.FC = () => {
  const [pickerOpen, setPickerOpen] = useState(false);
  const [openExistingOpen, setOpenExistingOpen] = useState(false);

  useEffect(() => {
    const onOpenPicker = () => {
      setPickerOpen(true);
    };

    document.addEventListener('open-project-picker', onOpenPicker);
    
    return () => {
      document.removeEventListener('open-project-picker', onOpenPicker);
    };
  }, []);

  return (
    <>
      <ProjectPickerModal 
        open={pickerOpen} 
        onOpenChange={setPickerOpen} 
        onOpenExisting={() => {
          setPickerOpen(false);
          setOpenExistingOpen(true);
        }}
      />
      <OpenExistingProjectModal 
        open={openExistingOpen} 
        onOpenChange={setOpenExistingOpen} 
        onBack={() => {
          setOpenExistingOpen(false);
          setPickerOpen(true);
        }}
      />
    </>
  );
};
